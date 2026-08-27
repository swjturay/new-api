package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setBusinessFeatureFlags(t *testing.T, referralEnabled, sidebarEnabled bool) {
	t.Helper()
	require.NoError(t, i18n.Init())
	previousReferral := common.ReferralProgramEnabled
	previousSidebar := common.UserSidebarCustomizationEnabled
	common.ReferralProgramEnabled = referralEnabled
	common.UserSidebarCustomizationEnabled = sidebarEnabled
	t.Cleanup(func() {
		common.ReferralProgramEnabled = previousReferral
		common.UserSidebarCustomizationEnabled = previousSidebar
	})
}

func featureTestContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, recorder
}

func TestReferralEndpointsReturnForbiddenWhenProgramDisabled(t *testing.T) {
	setBusinessFeatureFlags(t, false, false)

	tests := []struct {
		name    string
		method  string
		target  string
		body    string
		handler gin.HandlerFunc
	}{
		{name: "affiliate code", method: http.MethodGet, target: "/api/user/aff", handler: GetAffCode},
		{name: "reward transfer", method: http.MethodPost, target: "/api/user/aff_transfer", body: `{"quota":1}`, handler: TransferAffQuota},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, recorder := featureTestContext(test.method, test.target, test.body)

			test.handler(c)

			assert.Equal(t, http.StatusForbidden, recorder.Code)
			assert.Contains(t, recorder.Body.String(), `"success":false`)
		})
	}
}

func TestSidebarUpdateReturnsForbiddenWhenCustomizationDisabled(t *testing.T) {
	setBusinessFeatureFlags(t, false, false)
	c, recorder := featureTestContext(http.MethodPut, "/api/user/self", `{"sidebar_modules":"{}"}`)

	UpdateSelf(c)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":false`)
}

func TestDisabledFeaturesAreMaskedFromSelfUserData(t *testing.T) {
	setBusinessFeatureFlags(t, false, false)
	user := &model.User{
		Id:              7,
		Role:            common.RoleCommonUser,
		AffCode:         "saved-code",
		AffCount:        3,
		AffQuota:        400,
		AffHistoryQuota: 900,
		InviterId:       2,
	}
	user.SetSetting(dto.UserSetting{SidebarModules: `{"personal":{"enabled":false}}`})

	data := buildSelfUserData(user)

	assert.Empty(t, data["aff_code"])
	assert.Equal(t, 0, data["aff_count"])
	assert.Equal(t, 0, data["aff_quota"])
	assert.Equal(t, 0, data["aff_history_quota"])
	assert.Equal(t, 0, data["inviter_id"])
	assert.Empty(t, data["sidebar_modules"])
	assert.NotContains(t, data["setting"], "personal")
	permissions, ok := data["permissions"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, false, permissions["sidebar_settings"])
}

func TestResolveInviterIDIgnoresCodeWhenProgramDisabled(t *testing.T) {
	setBusinessFeatureFlags(t, false, false)
	assert.Zero(t, resolveInviterID("saved-code"))
}

func TestGetStatusPublishesBusinessFeatureFlags(t *testing.T) {
	setBusinessFeatureFlags(t, true, true)
	previousOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		common.OptionMap = previousOptionMap
	})
	c, recorder := featureTestContext(http.MethodGet, "/api/status", "")

	GetStatus(c)

	var payload struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	assert.Equal(t, true, payload.Data["referral_program_enabled"])
	assert.Equal(t, true, payload.Data["user_sidebar_customization_enabled"])
}
