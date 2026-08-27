package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func requireBusinessFeature(c *gin.Context, enabled bool) bool {
	if enabled {
		return true
	}
	c.JSON(http.StatusForbidden, gin.H{
		"success": false,
		"message": i18n.T(c, i18n.MsgFeatureDisabled),
	})
	return false
}

func resolveInviterID(affiliateCode string) int {
	if !common.ReferralProgramEnabled {
		return 0
	}
	affiliateCode = strings.TrimSpace(affiliateCode)
	if affiliateCode == "" {
		return 0
	}
	inviterID, _ := model.GetUserIdByAffCode(affiliateCode)
	return inviterID
}
