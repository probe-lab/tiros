package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunAWSConfig_String(t *testing.T) {
	rawsc := DefaultRunAWSConfig
	rawsc.DatabasePassword = "PASSWORD"

	assert.False(t, strings.Contains(rawsc.String(), "PASSWORD"))
}
