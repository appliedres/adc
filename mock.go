package adc

import (
	"context"
	"crypto/tls"
	"errors"
	"slices"
	"time"

	"github.com/go-ldap/ldap/v3"
)

// Entry attribute name, that helps match entry to provided request.
const mockFiltersAttribute = "filtersToFind"

// Mock client. Implements ldap client interface.
var _ ldap.Client = (*mockClient)(nil)

type mockClient struct {
	entries map[string]*ldap.Entry
}

// Extended implements ldap.Client.
func (cl *mockClient) Extended(*ldap.ExtendedRequest) (*ldap.ExtendedResponse, error) {
	panic("unimplemented")
}

// Initializes new mock client.
func newMockClient(cfg *Config, opts ...Option) *Client {
	cl := New(cfg, opts...)
	cl.mockMode = true
	return cl
}

// Initializes new mock client with mock data.
func mockConnection() (*mockClient, error) {
	cl := &mockClient{
		entries: map[string]*ldap.Entry{
			"user1": {
				DN: "OU=user1,DC=company,DC=com",
				Attributes: []*ldap.EntryAttribute{
					{Name: "sAMAccountName", Values: []string{"user1"}},
					{Name: mockFiltersAttribute, Values: []string{
						"(&(objectClass=person)(sAMAccountName=user1))",
						"(&(objectClass=person)(distinguishedName=OU=user1,DC=company,DC=com))",
						"(&(objectCategory=person)(memberOf=OU=group1,DC=company,DC=com))",
						"customFilterToSearchUser",
					}},
				},
			},
			"user2": {
				DN: "OU=user2,DC=company,DC=com",
				Attributes: []*ldap.EntryAttribute{
					{Name: "sAMAccountName", Values: []string{"user2"}},
					{Name: mockFiltersAttribute, Values: []string{
						"(&(objectClass=person)(sAMAccountName=user2))",
						"(&(objectClass=person)(distinguishedName=OU=user2,DC=company,DC=com))",
						"(&(objectCategory=person)(memberOf=OU=group2,DC=company,DC=com))",
					}},
				},
			},
			"userToAdd": {
				DN: "OU=userToAdd,DC=company,DC=com",
				Attributes: []*ldap.EntryAttribute{
					{Name: "sAMAccountName", Values: []string{"userToAdd"}},
					{Name: mockFiltersAttribute, Values: []string{
						"(&(objectClass=person)(sAMAccountName=userToAdd))",
						"(&(objectClass=person)(distinguishedName=OU=userToAdd,DC=company,DC=com))",
						"(&(objectCategory=person)(memberOf=OU=group2,DC=company,DC=com))",
					}},
				},
			},
			"group1": {
				DN: "OU=group1,DC=company,DC=com",
				Attributes: []*ldap.EntryAttribute{
					{Name: "sAMAccountName", Values: []string{"group1"}},
					{Name: mockFiltersAttribute, Values: []string{
						"(&(objectClass=group)(sAMAccountName=group1))",
						"(&(objectClass=group)(distinguishedName=OU=group1,DC=company,DC=com))",
						"(&(objectClass=group)(member=OU=user1,DC=company,DC=com))",
						"customFilterToSearchGroup",
					}},
				},
			},
			"group2": {
				DN: "OU=group2,DC=company,DC=com",
				Attributes: []*ldap.EntryAttribute{
					{Name: "sAMAccountName", Values: []string{"group2"}},
					{Name: mockFiltersAttribute, Values: []string{
						"(&(objectClass=group)(sAMAccountName=group2))",
						"(&(objectClass=group)(distinguishedName=OU=group2,DC=company,DC=com))",
						"(&(objectClass=group)(member=OU=user2,DC=company,DC=com))",
					}},
				},
			},
			"entryForErr": {
				DN: "OU=entryForErr,DC=company,DC=com",
				Attributes: []*ldap.EntryAttribute{
					{Name: "sAMAccountName", Values: []string{"entryForErr"}},
					{Name: mockFiltersAttribute, Values: []string{
						"(&(objectClass=person)(sAMAccountName=entryForErr))",
						"(&(objectClass=person)(distinguishedName=OU=entryForErr,DC=company,DC=com))",
						"(&(objectClass=group)(sAMAccountName=entryForErr))",
						"(&(objectClass=group)(distinguishedName=OU=entryForErr,DC=company,DC=com))",
						"(&(objectCategory=person)(memberOf=OU=groupWithErrMember,DC=company,DC=com))",
					}},
				},
			},
			"groupWithErrMember": {
				DN: "OU=groupWithErrMember,DC=company,DC=com",
				Attributes: []*ldap.EntryAttribute{
					{Name: "sAMAccountName", Values: []string{"groupWithErrMember"}},
					{Name: mockFiltersAttribute, Values: []string{
						"(&(objectClass=group)(sAMAccountName=groupWithErrMember))",
						"(&(objectClass=group)(distinguishedName=OU=groupWithErrMember,DC=company,DC=com))",
						"(&(objectClass=group)(member=OU=entryForErr,DC=company,DC=com))",
					}},
				},
			},
			"userToReconnect": {
				DN: "OU=userToReconnect,DC=company,DC=com",
				Attributes: []*ldap.EntryAttribute{
					{Name: "sAMAccountName", Values: []string{"userToReconnect"}},
					{Name: mockFiltersAttribute, Values: []string{
						"(&(objectClass=person)(sAMAccountName=userToReconnect))",
						"(&(objectClass=person)(distinguishedName=OU=userToReconnect,DC=company,DC=com))",
					}},
				},
			},
			"notUniq1": {
				DN: "OU=notUniq,DC=company,DC=com",
				Attributes: []*ldap.EntryAttribute{
					{Name: "sAMAccountName", Values: []string{"notUniq"}},
					{Name: mockFiltersAttribute, Values: []string{
						"(&(objectClass=person)(sAMAccountName=notUniq))",
						"(&(objectClass=person)(distinguishedName=OU=notUniq,DC=company,DC=com))",
						"(&(objectClass=group)(sAMAccountName=notUniq))",
						"(&(objectClass=group)(distinguishedName=OU=notUniq,DC=company,DC=com))",
					}},
				},
			},
			"notUniq2": {
				DN: "OU=notUniq,DC=company,DC=com",
				Attributes: []*ldap.EntryAttribute{
					{Name: "sAMAccountName", Values: []string{"notUniq"}},
					{Name: mockFiltersAttribute, Values: []string{
						"(&(objectClass=person)(sAMAccountName=notUniq))",
						"(&(objectClass=person)(distinguishedName=OU=notUniq,DC=company,DC=com))",
						"(&(objectClass=group)(sAMAccountName=notUniq))",
						"(&(objectClass=group)(distinguishedName=OU=notUniq,DC=company,DC=com))",
					}},
				},
			},
		},
	}

	return cl, nil
}

func (cl *mockClient) getEntryByDn(dn string) *ldap.Entry {
	for _, entry := range cl.entries {
		if entry.DN == dn {
			return entry
		}
	}
	return nil
}

func (cl *mockClient) getEntriesByFilter(filter string) ([]*ldap.Entry, error) {
	var result []*ldap.Entry
	for id, entry := range cl.entries {
		filters := entry.GetAttributeValues(mockFiltersAttribute)
		if slices.Contains(filters, filter) {
			if id == "entryForErr" {
				return nil, errors.New("error for tests")
			}
			if id == "userToReconnect" {
				return nil, ldap.NewError(200, errors.New("connection error"))
			}
			result = append(result, entry)
		}
	}
	return result, nil
}

func (cl *mockClient) Start() {}

func (cl *mockClient) StartTLS(*tls.Config) error { return nil }

func (cl *mockClient) Close() error { return nil }

func (cl *mockClient) GetLastError() error { return nil }

func (cl *mockClient) IsClosing() bool { return false }

func (cl *mockClient) SetTimeout(time.Duration) {}

func (cl *mockClient) TLSConnectionState() (tls.ConnectionState, bool) {
	return tls.ConnectionState{}, true
}

var (
	validMockBind     = &BindAccount{DN: "validUser", Password: "validPass"}
	reconnectMockBind = &BindAccount{DN: "OU=userToReconnect,DC=company,DC=com", Password: "validPass"}
)

func (cl *mockClient) Bind(username, password string) error {
	if username == validMockBind.DN && password == validMockBind.Password {
		return nil
	}
	return errors.New("unauthorised")
}

func (cl *mockClient) UnauthenticatedBind(username string) error {
	return nil
}

func (cl *mockClient) SimpleBind(*ldap.SimpleBindRequest) (*ldap.SimpleBindResult, error) {
	return nil, nil
}

func (cl *mockClient) ExternalBind() error { return nil }

func (cl *mockClient) NTLMUnauthenticatedBind(domain, username string) error {
	return nil
}

func (cl *mockClient) Unbind() error { return nil }

func (cl *mockClient) Add(*ldap.AddRequest) error { return nil }

func (cl *mockClient) Del(*ldap.DelRequest) error { return nil }

func (cl *mockClient) Modify(req *ldap.ModifyRequest) error {
	entry := cl.getEntryByDn(req.DN)
	if entry == nil {
		return errors.New("entry not found")
	}
	if entry.DN == cl.entries["entryForErr"].DN {
		return errors.New("error for tests")
	}
	return nil
}

func (cl *mockClient) ModifyDN(*ldap.ModifyDNRequest) error { return nil }

func (cl *mockClient) ModifyWithResult(*ldap.ModifyRequest) (*ldap.ModifyResult, error) {
	return nil, nil
}

func (cl *mockClient) Compare(dn, attribute, value string) (bool, error) {
	return true, nil
}

func (cl *mockClient) PasswordModify(*ldap.PasswordModifyRequest) (*ldap.PasswordModifyResult, error) {
	return nil, nil
}

func (cl *mockClient) Search(req *ldap.SearchRequest) (*ldap.SearchResult, error) {
	entries, err := cl.getEntriesByFilter(req.Filter)
	if err != nil {
		return nil, err
	}
	return &ldap.SearchResult{Entries: entries}, nil
}

func (cl *mockClient) SearchAsync(ctx context.Context, searchRequest *ldap.SearchRequest, bufferSize int) ldap.Response {
	return nil
}

func (cl *mockClient) SearchWithPaging(searchRequest *ldap.SearchRequest, pagingSize uint32) (*ldap.SearchResult, error) {
	return nil, nil
}

func (cl *mockClient) DirSync(searchRequest *ldap.SearchRequest, flags, maxAttrCount int64, cookie []byte) (*ldap.SearchResult, error) {
	return &ldap.SearchResult{}, nil
}

func (cl *mockClient) DirSyncAsync(ctx context.Context, searchRequest *ldap.SearchRequest, bufferSize int, flags, maxAttrCount int64, cookie []byte) ldap.Response {
	return nil
}

func (cl *mockClient) Syncrepl(ctx context.Context, searchRequest *ldap.SearchRequest, bufferSize int, mode ldap.ControlSyncRequestMode, cookie []byte, reloadHint bool) ldap.Response {
	return nil
}
