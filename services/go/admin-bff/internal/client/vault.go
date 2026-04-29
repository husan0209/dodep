package client

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	vaultapi "github.com/hashicorp/vault/api"
)

type VaultTransitClient struct {
	client *vaultapi.Client
	key    string
}

func NewVaultTransitClient() (*VaultTransitClient, error) {
	addr := strings.TrimSpace(os.Getenv("VAULT_ADDR"))
	token := strings.TrimSpace(os.Getenv("VAULT_TOKEN"))
	key := strings.TrimSpace(os.Getenv("VAULT_TRANSIT_KEY"))
	if addr == "" || token == "" || key == "" {
		return nil, ErrNotConfigured
	}
	if !strings.Contains(addr, "://") {
		addr = "http://" + addr
	}

	cfg := vaultapi.DefaultConfig()
	cfg.Address = addr
	cli, err := vaultapi.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create vault client: %w", err)
	}
	cli.SetToken(token)

	return &VaultTransitClient{client: cli, key: key}, nil
}

func (c *VaultTransitClient) Encrypt(ctx context.Context, plaintext string) (string, error) {
	if c == nil || c.client == nil {
		return "", ErrNotConfigured
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return "", err
		}
	}

	resp, err := c.client.Logical().Write("transit/encrypt/"+c.key, map[string]any{
		"plaintext": base64.StdEncoding.EncodeToString([]byte(plaintext)),
	})
	if err != nil {
		return "", fmt.Errorf("encrypt totp secret: %w", err)
	}
	if resp == nil || resp.Data == nil {
		return "", errors.New("empty vault encrypt response")
	}
	ciphertext, _ := resp.Data["ciphertext"].(string)
	if ciphertext == "" {
		return "", errors.New("missing vault ciphertext")
	}
	return ciphertext, nil
}

func (c *VaultTransitClient) Decrypt(ctx context.Context, ciphertext string) (string, error) {
	if c == nil || c.client == nil {
		return "", ErrNotConfigured
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return "", err
		}
	}

	resp, err := c.client.Logical().Write("transit/decrypt/"+c.key, map[string]any{
		"ciphertext": ciphertext,
	})
	if err != nil {
		return "", fmt.Errorf("decrypt totp secret: %w", err)
	}
	if resp == nil || resp.Data == nil {
		return "", errors.New("empty vault decrypt response")
	}
	plaintextB64, _ := resp.Data["plaintext"].(string)
	if plaintextB64 == "" {
		return "", errors.New("missing vault plaintext")
	}
	decoded, err := base64.StdEncoding.DecodeString(plaintextB64)
	if err != nil {
		return "", fmt.Errorf("decode vault plaintext: %w", err)
	}
	return string(decoded), nil
}
