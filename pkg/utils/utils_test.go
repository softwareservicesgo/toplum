package utils

import "testing"

func TestGenerateTokenPair(t *testing.T) {
	tk, _ := GenerateTokenPair(1)
	t.Log(tk)
}
