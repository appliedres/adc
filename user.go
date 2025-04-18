package adc

import (
	"errors"
	"fmt"

	"github.com/go-ldap/ldap/v3"
)

// Active Direcotry user.
type User struct {
	DN         string                 `json:"dn"`
	Id         string                 `json:"id"`
	Attributes map[string]interface{} `json:"attributes"`
	Groups     []UserGroup            `json:"groups"`
}

// Active Direcotry user group info.
type UserGroup struct {
	DN string `json:"dn"`
	Id string `json:"id"`
}

// Returns string attribute by attribute name.
// Returns empty string if attribute not exists or it can't be covnerted to string.
func (u *User) GetStringAttribute(name string) string {
	for att, val := range u.Attributes {
		if att == name {
			if s, ok := val.(string); ok {
				return s
			}
		}
	}
	return ""
}

type GetUserArgs struct {
	// User ID to search.
	Id string `json:"id"`
	// Optional User DN. Overwrites ID if provided in request.
	Dn string `json:"dn"`
	// Optional LDAP filter to search entry. Warning! provided Filter arg overwrites Id and Dn args usage.
	Filter string `json:"filter"`
	// Optional user attributes to overwrite attributes in client config.
	Attributes []string `json:"attributes"`
	// Skip search of user groups data. Can improve request time.
	SkipGroupsSearch bool `json:"skip_groups_search"`
}

func (args GetUserArgs) Validate() error {
	if args.Id == "" && args.Dn == "" && args.Filter == "" {
		return errors.New("neither of ID, DN or Filter provided")
	}
	return nil
}

func (cl *Client) ListUsers(args GetUserArgs, pageSize int, filter string) (*[]User, error) {
	req := &ldap.SearchRequest{
		BaseDN:       cl.Config.Users.SearchBase,
		Scope:        ldap.ScopeWholeSubtree,
		DerefAliases: ldap.NeverDerefAliases,
		TimeLimit:    int(cl.Config.Timeout.Seconds()),
		Filter:       cl.Config.Users.FilterByPerson,
		Attributes:   cl.Config.Users.Attributes,
	}
	if args.Attributes != nil {
		req.Attributes = args.Attributes
	}
	if len(filter) > 0 {
		req.Filter = filter
	}

	control := ldap.NewControlPaging(uint32(pageSize))
	var entries []*ldap.Entry

	for {
		req.Controls = []ldap.Control{control}

		sr, err := cl.ldap.Search(req)
		if err != nil {
			return nil, err
		}

		entries = append(entries, sr.Entries...)

		if sr.Controls == nil {
			break
		}

		pagingControl, ok := sr.Controls[0].(*ldap.ControlPaging)
		if !ok {
			break
		}

		if len(pagingControl.Cookie) == 0 {
			break
		}

		control.SetCookie(pagingControl.Cookie)
	}

	if entries == nil {
		return nil, nil
	}

	var results []User
	for _, entry := range entries {
		result := &User{
			DN:         entry.DN,
			Id:         entry.GetAttributeValue(cl.Config.Users.IdAttribute),
			Attributes: make(map[string]interface{}, len(entry.Attributes)),
		}
		for _, a := range entry.Attributes {
			result.Attributes[a.Name] = entry.GetAttributeValue(a.Name)
		}
		results = append(results, *result)
	}
	return &results, nil
}

func (cl *Client) GetUser(args GetUserArgs) (*User, error) {
	if err := args.Validate(); err != nil {
		return nil, err
	}

	var filter string
	if args.Filter != "" {
		filter = args.Filter
	} else {
		filter = fmt.Sprintf(cl.Config.Users.FilterById, args.Id)
		if args.Dn != "" {
			filter = fmt.Sprintf(cl.Config.Users.FilterByDn, ldap.EscapeFilter(args.Dn))
		}
	}

	req := &ldap.SearchRequest{
		BaseDN:       cl.Config.Users.SearchBase,
		Scope:        ldap.ScopeWholeSubtree,
		DerefAliases: ldap.NeverDerefAliases,
		TimeLimit:    int(cl.Config.Timeout.Seconds()),
		Filter:       filter,
		Attributes:   cl.Config.Users.Attributes,
	}
	if args.Attributes != nil {
		req.Attributes = args.Attributes
	}

	entry, err := cl.searchEntry(req)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}

	result := &User{
		DN:         entry.DN,
		Id:         entry.GetAttributeValue(cl.Config.Users.IdAttribute),
		Attributes: make(map[string]interface{}, len(entry.Attributes)),
	}
	for _, a := range entry.Attributes {
		result.Attributes[a.Name] = entry.GetAttributeValue(a.Name)
	}

	if !args.SkipGroupsSearch {
		groups, err := cl.getUserGroups(entry.DN)
		if err != nil {
			return nil, fmt.Errorf("can't get user groups: %s", err.Error())
		}
		result.Groups = groups
	}

	return result, nil
}

func (cl *Client) CreateUser(dn string, userAttrs []ldap.Attribute) error {
	addReq := ldap.NewAddRequest(dn, []ldap.Control{})
	addReq.Attributes = userAttrs

	return cl.addRequest(addReq)
}

func (cl *Client) DeleteUser(dn string) error {
	delReq := ldap.NewDelRequest(dn, []ldap.Control{})

	return cl.deleteRequest(delReq)
}

func (cl *Client) SetPassword(dn string, newPassword string, mustChange bool) error {
	err := cl.modifyPassword(dn, newPassword)
	if err != nil {
		return err
	}
	if !mustChange {
		return nil
	}
	return cl.updateAttribute(dn, "pwdLastSet", []string{"0"})
}

func (cl *Client) UpdateUser(dn string, userAttrs []ldap.Attribute) error {
	modReq := ldap.NewModifyRequest(dn, []ldap.Control{})
	for _, a := range userAttrs {
		modReq.Replace(a.Type, a.Vals)
	}
	return cl.modifyRequest(modReq)
}

func (cl *Client) getUserGroups(dn string) ([]UserGroup, error) {
	req := &ldap.SearchRequest{
		BaseDN:       cl.Config.Groups.SearchBase,
		Scope:        ldap.ScopeWholeSubtree,
		DerefAliases: ldap.NeverDerefAliases,
		TimeLimit:    int(cl.Config.Timeout.Seconds()),
		Filter:       fmt.Sprintf(cl.Config.Users.FilterGroupsByDn, ldap.EscapeFilter(dn)),
		Attributes:   []string{cl.Config.Groups.IdAttribute},
	}
	entries, err := cl.searchEntries(req)
	if err != nil {
		return nil, err
	}
	var result []UserGroup
	for _, e := range entries {
		result = append(result, UserGroup{
			DN: e.DN,
			Id: e.GetAttributeValue(cl.Config.Groups.IdAttribute),
		})
	}
	return result, nil
}

func (u *User) IsGroupMember(groupId string) bool {
	for _, g := range u.Groups {
		if g.Id == groupId {
			return true
		}
	}
	return false
}

// Returns list of user groups DNs.
func (u *User) GroupsDn() []string {
	var result []string
	for _, g := range u.Groups {
		result = append(result, g.DN)
	}
	return result
}

// Returns list of user groups IDs.
func (u *User) GroupsId() []string {
	var result []string
	for _, g := range u.Groups {
		result = append(result, g.Id)
	}
	return result
}
