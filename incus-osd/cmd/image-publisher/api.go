package main

import (
	"time"
)

type updateFull struct {
	update

	URL string `json:"url,omitempty"`
}

type update struct {
	Format string `json:"format"`

	Channel     string       `json:"channel"`
	Files       []updateFile `json:"files"`
	Origin      string       `json:"origin"`
	PublishedAt time.Time    `json:"published_at"`
	Severity    string       `json:"severity"`
	Version     string       `json:"version"`
}

type updateFile struct {
	Architecture string `json:"architecture"`
	Component    string `json:"component"`
	Filename     string `json:"filename"`
	Sha256       string `json:"sha256"`
	Size         int64  `json:"size"`
	Type         string `json:"type"`
}

type index struct {
	Format string `json:"format"`

	Updates []updateFull `json:"updates"`
}

var (
	updateFileComponentOS                  = "os"
	updateFileComponentIncus               = "incus"
	updateFileComponentDebug               = "debug"
	updateFileTypeImageRaw                 = "image-raw"
	updateFileTypeImageISO                 = "image-iso"
	updateFileTypeUpdateEFI                = "update-efi"
	updateFileTypeUpdateUsr                = "update-usr"
	updateFileTypeUpdateUsrVerity          = "update-usr-verity"
	updateFileTypeUpdateUsrVeritySignature = "update-usr-verity-signature"
	updateFileTypeApplication              = "application"
)
