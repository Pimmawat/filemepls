package domain

import "errors"

var (
	ErrNotFound            = errors.New("domain: not found")
	ErrNotOwner            = errors.New("domain: user does not own this resource")
	ErrFileTooLarge        = errors.New("domain: file exceeds max allowed size")
	ErrInvalidSize         = errors.New("domain: file size must be greater than zero")
	ErrUnsupportedMime     = errors.New("domain: mime type not allowed")
	ErrEmptyHash           = errors.New("domain: file hash must not be empty")
	ErrInvalidHash         = errors.New("domain: file hash must be 64 lowercase hex characters")
	ErrShareExpired        = errors.New("domain: share link has expired")
	ErrDownloadLimitHit    = errors.New("domain: download limit reached")
	ErrInvalidPassword     = errors.New("domain: password does not match")
	ErrPasswordRequired    = errors.New("domain: share link requires a password")
	ErrInvalidVisibility   = errors.New("domain: invalid visibility value")
	ErrEmptyEmail          = errors.New("domain: user email must not be empty")
	ErrEmptyFolderName     = errors.New("domain: folder name must not be empty")
	ErrInvalidFolderName   = errors.New("domain: folder name must not contain a path separator")
	ErrCyclicMove          = errors.New("domain: cannot move a folder into its own descendant")
	ErrShareTargetRequired = errors.New("domain: share link must target exactly one of file or folder")
	ErrShareTargetMismatch = errors.New("domain: share link does not target the expected resource type")
)
