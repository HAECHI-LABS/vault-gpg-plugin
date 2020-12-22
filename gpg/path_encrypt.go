package gpg

import (
  "bytes"
  "context"
  "crypto"
  "encoding/base64"
  "fmt"
  "github.com/hashicorp/vault/sdk/framework"
  "github.com/hashicorp/vault/sdk/logical"
  "golang.org/x/crypto/openpgp"
  "golang.org/x/crypto/openpgp/packet"
  "golang.org/x/crypto/openpgp/armor"
  _ "golang.org/x/crypto/ripemd160"
  "io"
  "strings"
)

func pathEncrypt(b *backend) *framework.Path {
  return &framework.Path{
    Pattern: "encrypt/" + framework.GenericNameRegex("name") + framework.OptionalParamRegex("urlalgorithm"),
    Fields: map[string]*framework.FieldSchema{
      "name": {
        Type:        framework.TypeString,
        Description: "The key to use",
      },
      "plaintext": {
        Type:        framework.TypeString,
        Description: "The plaintext to encrypt",
      },
      "urlalgorithm": {
        Type:        framework.TypeString,
        Description: "Hash algorithm to use (POST URL parameter)",
      },
      "algorithm": {
        Type:    framework.TypeString,
        Default: "sha2-256",
        Description: `Hash algorithm to use (POST body parameter). Valid values are:

* sha2-224
* sha2-256
* sha2-384
* sha2-512

Defaults to "sha2-256".`,
      },
      "format": {
        Type:        framework.TypeString,
        Default:     "base64",
        Description: `Encoding format to use. Can be "base64" or "ascii-armor". Defaults to "base64".`,
      },
      "recipient_key": {
        Type:        framework.TypeString,
        Description: "The ASCII-armored GPG key of the recipient of the ciphertext.",
      },
    },
    Operations: map[logical.Operation]framework.OperationHandler{
      logical.UpdateOperation: &framework.PathOperation{
        Callback: b.pathEncryptWrite,
      },
    },
    HelpSynopsis:    pathEncryptHelpSyn,
    HelpDescription: pathEncryptHelpDesc,
  }
}

func (b *backend) pathEncryptWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
  plaintextB64 := data.Get("plaintext").(string)
  plaintext, err := base64.StdEncoding.DecodeString(plaintextB64)
  if err != nil {
    return logical.ErrorResponse(fmt.Sprintf("unable to decode plaintext as base64: %s", err)), logical.ErrInvalidRequest
  }

  config := packet.Config{}

  algorithm := data.Get("urlalgorithm").(string)
  if algorithm == "" {
    algorithm = data.Get("algorithm").(string)
  }
  switch algorithm {
  case "sha2-224":
    config.DefaultHash = crypto.SHA224
  case "sha2-256":
    config.DefaultHash = crypto.SHA256
  case "sha2-384":
    config.DefaultHash = crypto.SHA384
  case "sha2-512":
    config.DefaultHash = crypto.SHA512
  default:
    return logical.ErrorResponse(fmt.Sprintf("unsupported algorithm %s", algorithm)), nil
  }

  format := data.Get("format").(string)
  switch format {
  case "base64":
  case "ascii-armor":
  default:
    return logical.ErrorResponse(fmt.Sprintf("unsupported encoding format %s; must be \"base64\" or \"ascii-armor\"", format)), nil
  }

  recipientKey := data.Get("recipient_key").(string)
  if recipientKey == "" {
    return logical.ErrorResponse("recipient_key not exist"), logical.ErrInvalidRequest
  }
  el, err := openpgp.ReadArmoredKeyRing(strings.NewReader(recipientKey))
  if err != nil {
    return logical.ErrorResponse(err.Error()), logical.ErrInvalidRequest
  }
  recipientKeyList := []*openpgp.Entity{el[0]}
  if err != nil {
    return nil, err
  }

  entry, err := b.key(ctx, req.Storage, data.Get("name").(string))
  if err != nil {
    return nil, err
  }
  if entry == nil {
    return logical.ErrorResponse("key not found"), logical.ErrInvalidRequest
  }
  entity, err := b.entity(entry)
  if err != nil {
    return nil, err
  }

  ciphertext := new(bytes.Buffer)
  var ciphertextEncoder io.WriteCloser
  switch format {
  case "ascii-armor":
    encoder, err := armor.Encode(ciphertext, "PGP MESSAGE", nil)
    if err != nil {
      return nil, err
    }
    ciphertextEncoder = encoder;
  case "base64":
    ciphertextEncoder = base64.NewEncoder(base64.StdEncoding, ciphertext)
  }

  w, err := openpgp.Encrypt(ciphertextEncoder, recipientKeyList, entity, nil, &config)
  if err != nil {
    return nil, err
  }
  _, err = w.Write([]byte(plaintext))
  if err != nil {
    return nil, err
  }
  err = w.Close()
  if err != nil {
    return nil, err
  }
  err = ciphertextEncoder.Close()
  if err != nil {
    return nil, err
  }

  return &logical.Response{
    Data: map[string]interface{}{
      "ciphertext": ciphertext.String(),
    },
  }, nil
}

const pathEncryptHelpSyn = "Encrypt a plaintext value using the named GPG key"
const pathEncryptHelpDesc = `
This path uses the named GPG key from the request path to encrypt a user
provided plaintext. The ciphertext is returned base64 encoded.
`