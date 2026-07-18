package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupInvitationTestState(t *testing.T) {
	t.Helper()
	truncateTables(t)
	require.NoError(t, DB.Exec("DELETE FROM users").Error)

	oldRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.RedisEnabled = oldRedisEnabled
	})
}

func TestResolveInvitationWithTxRequiresValidUnusedCode(t *testing.T) {
	setupInvitationTestState(t)

	inviter := User{
		Username: "inviter",
		Password: "password",
		Status:   common.UserStatusEnabled,
		AffCode:  "single-use-code",
	}
	require.NoError(t, DB.Create(&inviter).Error)

	err := DB.Transaction(func(tx *gorm.DB) error {
		_, err := ResolveInvitationWithTx(tx, "", true)
		return err
	})
	require.ErrorIs(t, err, ErrInvitationRequired)

	err = DB.Transaction(func(tx *gorm.DB) error {
		_, err := ResolveInvitationWithTx(tx, "missing-code", true)
		return err
	})
	require.ErrorIs(t, err, ErrInvitationInvalid)

	var inviterId int
	invited := User{
		Username: "invited",
		Password: "password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		resolvedId, err := ResolveInvitationWithTx(tx, "single-use-code", true)
		if err != nil {
			return err
		}
		inviterId = resolvedId
		return invited.InsertWithTx(tx, inviterId)
	}))

	assert.Equal(t, inviter.Id, inviterId)
	assert.Equal(t, inviter.Id, invited.InviterId)
	require.Len(t, invited.AffCode, invitationCodeLength)

	var refreshedInviter User
	require.NoError(t, DB.First(&refreshedInviter, inviter.Id).Error)
	assert.NotEqual(t, "single-use-code", refreshedInviter.AffCode)
	require.Len(t, refreshedInviter.AffCode, invitationCodeLength)

	err = DB.Transaction(func(tx *gorm.DB) error {
		_, err := ResolveInvitationWithTx(tx, "single-use-code", true)
		return err
	})
	require.ErrorIs(t, err, ErrInvitationInvalid)
}

func TestResolveInvitationWithTxRollsBackCodeConsumption(t *testing.T) {
	setupInvitationTestState(t)

	inviter := User{
		Username: "rollback-inviter",
		Password: "password",
		Status:   common.UserStatusEnabled,
		AffCode:  "rollback-code",
	}
	require.NoError(t, DB.Create(&inviter).Error)

	wantErr := errors.New("registration failed")
	err := DB.Transaction(func(tx *gorm.DB) error {
		_, err := ResolveInvitationWithTx(tx, "rollback-code", true)
		if err != nil {
			return err
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)

	var refreshed User
	require.NoError(t, DB.First(&refreshed, inviter.Id).Error)
	assert.Equal(t, "rollback-code", refreshed.AffCode)
}
