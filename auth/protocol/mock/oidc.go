// Code generated by MockGen. DO NOT EDIT.
// Source: oidc.go

// Package mock_protocol is a generated GoMock package.
package mock_protocol

import (
	context "context"
	http "net/http"
	reflect "reflect"

	protocol "github.com/88labs/go-utils/auth/protocol"
	oidc "github.com/coreos/go-oidc"
	gomock "github.com/golang/mock/gomock"
	oauth2 "golang.org/x/oauth2"
)

// MockOAuth2Config is a mock of OAuth2Config interface.
type MockOAuth2Config struct {
	ctrl     *gomock.Controller
	recorder *MockOAuth2ConfigMockRecorder
}

// MockOAuth2ConfigMockRecorder is the mock recorder for MockOAuth2Config.
type MockOAuth2ConfigMockRecorder struct {
	mock *MockOAuth2Config
}

// NewMockOAuth2Config creates a new mock instance.
func NewMockOAuth2Config(ctrl *gomock.Controller) *MockOAuth2Config {
	mock := &MockOAuth2Config{ctrl: ctrl}
	mock.recorder = &MockOAuth2ConfigMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockOAuth2Config) EXPECT() *MockOAuth2ConfigMockRecorder {
	return m.recorder
}

// AuthCodeURL mocks base method.
func (m *MockOAuth2Config) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
	m.ctrl.T.Helper()
	varargs := []interface{}{state}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "AuthCodeURL", varargs...)
	ret0, _ := ret[0].(string)
	return ret0
}

// AuthCodeURL indicates an expected call of AuthCodeURL.
func (mr *MockOAuth2ConfigMockRecorder) AuthCodeURL(state interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{state}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AuthCodeURL", reflect.TypeOf((*MockOAuth2Config)(nil).AuthCodeURL), varargs...)
}

// Exchange mocks base method.
func (m *MockOAuth2Config) Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx, code}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Exchange", varargs...)
	ret0, _ := ret[0].(*oauth2.Token)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Exchange indicates an expected call of Exchange.
func (mr *MockOAuth2ConfigMockRecorder) Exchange(ctx, code interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, code}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Exchange", reflect.TypeOf((*MockOAuth2Config)(nil).Exchange), varargs...)
}

// TokenSource mocks base method.
func (m *MockOAuth2Config) TokenSource(ctx context.Context, t *oauth2.Token) oauth2.TokenSource {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TokenSource", ctx, t)
	ret0, _ := ret[0].(oauth2.TokenSource)
	return ret0
}

// TokenSource indicates an expected call of TokenSource.
func (mr *MockOAuth2ConfigMockRecorder) TokenSource(ctx, t interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TokenSource", reflect.TypeOf((*MockOAuth2Config)(nil).TokenSource), ctx, t)
}

// MockIDTokenVerifier is a mock of IDTokenVerifier interface.
type MockIDTokenVerifier struct {
	ctrl     *gomock.Controller
	recorder *MockIDTokenVerifierMockRecorder
}

// MockIDTokenVerifierMockRecorder is the mock recorder for MockIDTokenVerifier.
type MockIDTokenVerifierMockRecorder struct {
	mock *MockIDTokenVerifier
}

// NewMockIDTokenVerifier creates a new mock instance.
func NewMockIDTokenVerifier(ctrl *gomock.Controller) *MockIDTokenVerifier {
	mock := &MockIDTokenVerifier{ctrl: ctrl}
	mock.recorder = &MockIDTokenVerifierMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockIDTokenVerifier) EXPECT() *MockIDTokenVerifierMockRecorder {
	return m.recorder
}

// Verify mocks base method.
func (m *MockIDTokenVerifier) Verify(ctx context.Context, rawIDToken string) (*oidc.IDToken, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Verify", ctx, rawIDToken)
	ret0, _ := ret[0].(*oidc.IDToken)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Verify indicates an expected call of Verify.
func (mr *MockIDTokenVerifierMockRecorder) Verify(ctx, rawIDToken interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Verify", reflect.TypeOf((*MockIDTokenVerifier)(nil).Verify), ctx, rawIDToken)
}

// MockStateManager is a mock of StateManager interface.
type MockStateManager struct {
	ctrl     *gomock.Controller
	recorder *MockStateManagerMockRecorder
}

// MockStateManagerMockRecorder is the mock recorder for MockStateManager.
type MockStateManagerMockRecorder struct {
	mock *MockStateManager
}

// NewMockStateManager creates a new mock instance.
func NewMockStateManager(ctrl *gomock.Controller) *MockStateManager {
	mock := &MockStateManager{ctrl: ctrl}
	mock.recorder = &MockStateManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockStateManager) EXPECT() *MockStateManagerMockRecorder {
	return m.recorder
}

// Issue mocks base method.
func (m *MockStateManager) Issue(w http.ResponseWriter, r *http.Request) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Issue", w, r)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Issue indicates an expected call of Issue.
func (mr *MockStateManagerMockRecorder) Issue(w, r interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Issue", reflect.TypeOf((*MockStateManager)(nil).Issue), w, r)
}

// Verify mocks base method.
func (m *MockStateManager) Verify(w http.ResponseWriter, r *http.Request) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Verify", w, r)
	ret0, _ := ret[0].(error)
	return ret0
}

// Verify indicates an expected call of Verify.
func (mr *MockStateManagerMockRecorder) Verify(w, r interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Verify", reflect.TypeOf((*MockStateManager)(nil).Verify), w, r)
}

// MockUserInfoProvider is a mock of UserInfoProvider interface.
type MockUserInfoProvider struct {
	ctrl     *gomock.Controller
	recorder *MockUserInfoProviderMockRecorder
}

// MockUserInfoProviderMockRecorder is the mock recorder for MockUserInfoProvider.
type MockUserInfoProviderMockRecorder struct {
	mock *MockUserInfoProvider
}

// NewMockUserInfoProvider creates a new mock instance.
func NewMockUserInfoProvider(ctrl *gomock.Controller) *MockUserInfoProvider {
	mock := &MockUserInfoProvider{ctrl: ctrl}
	mock.recorder = &MockUserInfoProviderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockUserInfoProvider) EXPECT() *MockUserInfoProviderMockRecorder {
	return m.recorder
}

// UserInfo mocks base method.
func (m *MockUserInfoProvider) UserInfo(ctx context.Context, tokenSource oauth2.TokenSource) (*protocol.UserInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UserInfo", ctx, tokenSource)
	ret0, _ := ret[0].(*protocol.UserInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UserInfo indicates an expected call of UserInfo.
func (mr *MockUserInfoProviderMockRecorder) UserInfo(ctx, tokenSource interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UserInfo", reflect.TypeOf((*MockUserInfoProvider)(nil).UserInfo), ctx, tokenSource)
}
