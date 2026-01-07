package model

import (
	"bytes"
	"os"
	"time"

	"github.com/fullsailor/pkcs7"
	plist "howett.net/plist"
)

type MobileProvisioningProfile struct {
	AppIDName      string
	CreationDate   time.Time
	ExpirationDate time.Time
	Name           string
	TeamName       string
	UUID           string
	Version        int
}

func NewMobileProvisioningProfile(data []byte) (*MobileProvisioningProfile, error) {
	p7, err := pkcs7.Parse(data)
	if err != nil {
		return nil, err
	}

	decoder := plist.NewDecoder(bytes.NewReader(p7.Content))

	var profile map[string]interface{}
	err = decoder.Decode(&profile)
	if err != nil {
		return nil, err
	}

	return &MobileProvisioningProfile{
		AppIDName:      profile["AppIDName"].(string),
		CreationDate:   profile["CreationDate"].(time.Time),
		ExpirationDate: profile["ExpirationDate"].(time.Time),
		Name:           profile["Name"].(string),
		TeamName:       profile["TeamName"].(string),
		UUID:           profile["UUID"].(string),
		Version:        int(profile["Version"].(uint64)),
	}, nil
}

func ParseMobileProvisioningProfileFile(path string) (*MobileProvisioningProfile, error) {
	if f, err := os.Stat(path); err != nil || f.Size() == 0 {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseMobileProvisioningProfile(data)
}

func ParseMobileProvisioningProfile(data []byte) (*MobileProvisioningProfile, error) {
	p7, err := pkcs7.Parse(data)
	if err != nil {
		return nil, err
	}

	decoder := plist.NewDecoder(bytes.NewReader(p7.Content))

	var profile map[string]interface{}
	err = decoder.Decode(&profile)
	if err != nil {
		return nil, err
	}

	return &MobileProvisioningProfile{
		AppIDName:      profile["AppIDName"].(string),
		CreationDate:   profile["CreationDate"].(time.Time),
		ExpirationDate: profile["ExpirationDate"].(time.Time),
		Name:           profile["Name"].(string),
		TeamName:       profile["TeamName"].(string),
		UUID:           profile["UUID"].(string),
		Version:        int(profile["Version"].(uint64)),
	}, nil
}
