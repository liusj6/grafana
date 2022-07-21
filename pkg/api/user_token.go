package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/grafana/grafana/pkg/api/dtos"
	"github.com/grafana/grafana/pkg/api/response"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/user"
	"github.com/grafana/grafana/pkg/util"
	"github.com/grafana/grafana/pkg/web"
	"github.com/ua-parser/uap-go/uaparser"
)

// swagger:route GET /user/auth-tokens signed_in_user getSignedInUserAuthTokens
//
// Auth tokens of the actual User.
//
// Return a list of all auth tokens (devices) that the actual user currently have logged in from.
//
// Responses:
// 200: getSignedInUserAuthTokensResponse
// 401: unauthorisedError
// 403: forbiddenError
// 500: internalServerError
func (hs *HTTPServer) GetUserAuthTokens(c *models.ReqContext) response.Response {
	return hs.getUserAuthTokensInternal(c, c.UserId)
}

// swagger:route POST /user/revoke-auth-token signed_in_user revokeSignedInAuthToken
//
// Revoke an auth token of the actual User.
//
// Revokes the given auth token (device) for the actual user. User of issued auth token (device) will no longer be logged in and will be required to authenticate again upon next activity.
//
// Responses:
// 200: okResponse
// 400: badRequestError
// 401: unauthorisedError
// 403: forbiddenError
// 500: internalServerError
func (hs *HTTPServer) RevokeUserAuthToken(c *models.ReqContext) response.Response {
	cmd := models.RevokeAuthTokenCmd{}
	if err := web.Bind(c.Req, &cmd); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}
	return hs.revokeUserAuthTokenInternal(c, c.UserId, cmd)
}

func (hs *HTTPServer) logoutUserFromAllDevicesInternal(ctx context.Context, userID int64) response.Response {
	userQuery := models.GetUserByIdQuery{Id: userID}

	if err := hs.SQLStore.GetUserById(ctx, &userQuery); err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			return response.Error(404, "User not found", err)
		}
		return response.Error(500, "Could not read user from database", err)
	}

	err := hs.AuthTokenService.RevokeAllUserTokens(ctx, userID)
	if err != nil {
		return response.Error(500, "Failed to logout user", err)
	}

	return response.JSON(http.StatusOK, util.DynMap{
		"message": "User logged out",
	})
}

func (hs *HTTPServer) getUserAuthTokensInternal(c *models.ReqContext, userID int64) response.Response {
	userQuery := models.GetUserByIdQuery{Id: userID}

	if err := hs.SQLStore.GetUserById(c.Req.Context(), &userQuery); err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			return response.Error(http.StatusNotFound, "User not found", err)
		} else if errors.Is(err, user.ErrCaseInsensitive) {
			return response.Error(http.StatusConflict,
				"User has conflicting login or email with another user. Please contact server admin", err)
		}

		return response.Error(http.StatusInternalServerError, "Failed to get user", err)
	}

	tokens, err := hs.AuthTokenService.GetUserTokens(c.Req.Context(), userID)
	if err != nil {
		return response.Error(500, "Failed to get user auth tokens", err)
	}

	result := []*dtos.UserToken{}
	for _, token := range tokens {
		isActive := false
		if c.UserToken != nil && c.UserToken.Id == token.Id {
			isActive = true
		}

		parser := uaparser.NewFromSaved()
		client := parser.Parse(token.UserAgent)

		osVersion := ""
		if client.Os.Major != "" {
			osVersion = client.Os.Major

			if client.Os.Minor != "" {
				osVersion = osVersion + "." + client.Os.Minor
			}
		}

		browserVersion := ""
		if client.UserAgent.Major != "" {
			browserVersion = client.UserAgent.Major

			if client.UserAgent.Minor != "" {
				browserVersion = browserVersion + "." + client.UserAgent.Minor
			}
		}

		createdAt := time.Unix(token.CreatedAt, 0)
		seenAt := time.Unix(token.SeenAt, 0)

		if token.SeenAt == 0 {
			seenAt = createdAt
		}

		result = append(result, &dtos.UserToken{
			Id:                     token.Id,
			IsActive:               isActive,
			ClientIp:               token.ClientIp,
			Device:                 client.Device.ToString(),
			OperatingSystem:        client.Os.Family,
			OperatingSystemVersion: osVersion,
			Browser:                client.UserAgent.Family,
			BrowserVersion:         browserVersion,
			CreatedAt:              createdAt,
			SeenAt:                 seenAt,
		})
	}

	return response.JSON(http.StatusOK, result)
}

func (hs *HTTPServer) revokeUserAuthTokenInternal(c *models.ReqContext, userID int64, cmd models.RevokeAuthTokenCmd) response.Response {
	userQuery := models.GetUserByIdQuery{Id: userID}
	if err := hs.SQLStore.GetUserById(c.Req.Context(), &userQuery); err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			return response.Error(404, "User not found", err)
		}
		return response.Error(500, "Failed to get user", err)
	}

	token, err := hs.AuthTokenService.GetUserToken(c.Req.Context(), userID, cmd.AuthTokenId)
	if err != nil {
		if errors.Is(err, models.ErrUserTokenNotFound) {
			return response.Error(404, "User auth token not found", err)
		}
		return response.Error(500, "Failed to get user auth token", err)
	}

	if c.UserToken != nil && c.UserToken.Id == token.Id {
		return response.Error(400, "Cannot revoke active user auth token", nil)
	}

	err = hs.AuthTokenService.RevokeToken(c.Req.Context(), token, false)
	if err != nil {
		if errors.Is(err, models.ErrUserTokenNotFound) {
			return response.Error(404, "User auth token not found", err)
		}
		return response.Error(500, "Failed to revoke user auth token", err)
	}

	return response.JSON(http.StatusOK, util.DynMap{
		"message": "User auth token revoked",
	})
}

// swagger:parameters updateSignedInUser
type UpdateSignedInUserParams struct {
	// To change the email, name, login, theme, provide another one.
	// in:body
	// required:true
	Body models.UpdateUserCommand `json:"body"`
}

// swagger:parameters userSetUsingOrg
type UserSetUsingOrgParams struct {
	// in:path
	// required:true
	OrgID int64 `json:"org_id"`
}

// swagger:parameters starDashboard
type StarDashboardParams struct {
	// in:path
	// required:true
	DashboardID string `json:"dashboard_id"`
}

// swagger:parameters unstarDashboard
type UnstarDashboardParams struct {
	// in:path
	// required:true
	DashboardID string `json:"dashboard_id"`
}

// swagger:parameters setHelpFlag
type SetHelpFlagParams struct {
	// in:path
	// required:true
	FlagID string `json:"flag_id"`
}

// swagger:parameters changeUserPassword
type ChangeUserPasswordParams struct {
	// To change the email, name, login, theme, provide another one.
	// in:body
	// required:true
	Body models.ChangeUserPasswordCommand `json:"body"`
}

// swagger:parameters revokeSignedInAuthToken
type RevokeSignedINAuthTokenCmdParams struct {
	// in:body
	// required:true
	Body models.RevokeAuthTokenCmd `json:"body"`
}

// swagger:response helpFlagResponse
type HelpFlagResponse struct {
	// The response message
	// in: body
	Body struct {
		HelpFlags1 int64  `json:"helpFlags1"`
		Message    string `json:"message"`
	} `json:"body"`
}

// swagger:response getSignedInUserAuthTokensResponse
type GetSignedInUserAuthTokensResponse struct {
	// in:body
	Body []*models.UserToken `json:"body"`
}
