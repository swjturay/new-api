package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/require"
)

func TestMultiKeyCredentialTraversalHonorsRequestExclusions(t *testing.T) {
	channel := &Channel{
		Key: "key-a\nkey-b\nkey-c",
		ChannelInfo: ChannelInfo{
			IsMultiKey: true,
			MultiKeyStatusList: map[int]int{
				1: common.ChannelStatusAutoDisabled,
			},
			MultiKeyMode: constant.MultiKeyModeRandom,
		},
	}

	key, index, err := channel.GetNextEnabledKeyExcluding(map[int]struct{}{0: {}})
	require.Nil(t, err)
	require.Equal(t, "key-c", key)
	require.Equal(t, 2, index)

	_, _, err = channel.GetNextEnabledKeyExcluding(map[int]struct{}{0: {}, 2: {}})
	require.NotNil(t, err)
	require.Equal(t, types.ErrorCodeChannelNoAvailableKey, err.GetErrorCode())
}

func TestEnabledKeyCountIgnoresPersistentlyDisabledCredentials(t *testing.T) {
	channel := &Channel{
		Key: "key-a\nkey-b",
		ChannelInfo: ChannelInfo{
			IsMultiKey:         true,
			MultiKeyStatusList: map[int]int{1: common.ChannelStatusAutoDisabled},
		},
	}
	require.Equal(t, 1, channel.EnabledKeyCount())
}
