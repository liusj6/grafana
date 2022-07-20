package login

import (
	"context"
	"testing"

	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/sqlstore/mockstore"
	"github.com/grafana/grafana/pkg/services/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoginUsingGrafanaDB(t *testing.T) {
	grafanaLoginScenario(t, "When login with non-existing user", func(sc *grafanaLoginScenarioContext) {
		sc.withNonExistingUser()
		err := loginUsingGrafanaDB(context.Background(), sc.loginUserQuery, sc.store)
		require.EqualError(t, err, user.ErrUserNotFound.Error())

		assert.False(t, sc.validatePasswordCalled)
		assert.Nil(t, sc.loginUserQuery.User)
	})

	grafanaLoginScenario(t, "When login with invalid credentials", func(sc *grafanaLoginScenarioContext) {
		sc.withInvalidPassword()
		err := loginUsingGrafanaDB(context.Background(), sc.loginUserQuery, sc.store)

		require.EqualError(t, err, ErrInvalidCredentials.Error())

		assert.True(t, sc.validatePasswordCalled)
		assert.Nil(t, sc.loginUserQuery.User)
	})

	grafanaLoginScenario(t, "When login with valid credentials", func(sc *grafanaLoginScenarioContext) {
		sc.withValidCredentials()
		err := loginUsingGrafanaDB(context.Background(), sc.loginUserQuery, sc.store)
		require.NoError(t, err)

		assert.True(t, sc.validatePasswordCalled)

		require.NotNil(t, sc.loginUserQuery.User)
		assert.Equal(t, sc.loginUserQuery.Username, sc.loginUserQuery.User.Login)
		assert.Equal(t, sc.loginUserQuery.Password, sc.loginUserQuery.User.Password)
	})

	grafanaLoginScenario(t, "When login with disabled user", func(sc *grafanaLoginScenarioContext) {
		sc.withDisabledUser()
		err := loginUsingGrafanaDB(context.Background(), sc.loginUserQuery, sc.store)
		require.EqualError(t, err, ErrUserDisabled.Error())

		assert.False(t, sc.validatePasswordCalled)
		assert.Nil(t, sc.loginUserQuery.User)
	})
}

type grafanaLoginScenarioContext struct {
	store                  *mockstore.SQLStoreMock
	loginUserQuery         *models.LoginUserQuery
	validatePasswordCalled bool
}

type grafanaLoginScenarioFunc func(c *grafanaLoginScenarioContext)

func grafanaLoginScenario(t *testing.T, desc string, fn grafanaLoginScenarioFunc) {
	t.Helper()

	t.Run(desc, func(t *testing.T) {
		origValidatePassword := validatePassword

		sc := &grafanaLoginScenarioContext{
			store: mockstore.NewSQLStoreMock(),
			loginUserQuery: &models.LoginUserQuery{
				Username:  "user",
				Password:  "pwd",
				IpAddress: "192.168.1.1:56433",
			},
			validatePasswordCalled: false,
		}

		t.Cleanup(func() {
			validatePassword = origValidatePassword
		})

		fn(sc)
	})
}

func mockPasswordValidation(valid bool, sc *grafanaLoginScenarioContext) {
	validatePassword = func(providedPassword string, userPassword string, userSalt string) error {
		sc.validatePasswordCalled = true

		if !valid {
			return ErrInvalidCredentials
		}

		return nil
	}
}

func (sc *grafanaLoginScenarioContext) getUserByLoginQueryReturns(usr *user.User) {
	sc.store.ExpectedUser = usr
	if usr == nil {
		sc.store.ExpectedError = user.ErrUserNotFound
	}
}

func (sc *grafanaLoginScenarioContext) withValidCredentials() {
	sc.getUserByLoginQueryReturns(&user.User{
		ID:       1,
		Login:    sc.loginUserQuery.Username,
		Password: sc.loginUserQuery.Password,
		Salt:     "salt",
	})
	mockPasswordValidation(true, sc)
}

func (sc *grafanaLoginScenarioContext) withNonExistingUser() {
	sc.getUserByLoginQueryReturns(nil)
}

func (sc *grafanaLoginScenarioContext) withInvalidPassword() {
	sc.getUserByLoginQueryReturns(&user.User{
		Password: sc.loginUserQuery.Password,
		Salt:     "salt",
	})
	mockPasswordValidation(false, sc)
}

func (sc *grafanaLoginScenarioContext) withDisabledUser() {
	sc.getUserByLoginQueryReturns(&user.User{
		IsDisabled: true,
	})
}
