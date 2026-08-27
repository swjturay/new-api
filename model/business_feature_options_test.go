package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBusinessFeatureOptionsDefaultToDisabled(t *testing.T) {
	previousReferral := common.ReferralProgramEnabled
	previousSidebar := common.UserSidebarCustomizationEnabled
	common.ReferralProgramEnabled = false
	common.UserSidebarCustomizationEnabled = false
	t.Cleanup(func() {
		common.ReferralProgramEnabled = previousReferral
		common.UserSidebarCustomizationEnabled = previousSidebar
	})

	InitOptionMap()

	assert.Equal(t, "false", common.OptionMap["ReferralProgramEnabled"])
	assert.Equal(t, "false", common.OptionMap["UserSidebarCustomizationEnabled"])
	require.NoError(t, updateOptionMap("ReferralProgramEnabled", "true"))
	require.NoError(t, updateOptionMap("UserSidebarCustomizationEnabled", "true"))
	assert.True(t, common.ReferralProgramEnabled)
	assert.True(t, common.UserSidebarCustomizationEnabled)
}
