package ipa

import (
	"image"

	"github.com/iineva/ipa-server/pkg/common"
)

type IPA struct {
	info *InfoPlist
	icon image.Image
	size int64
}

func (i *IPA) Name() string {
	return common.Def(i.info.CFBundleDisplayName, i.info.CFBundleName, i.info.CFBundleExecutable)
}

func (i *IPA) Version() string {
	return i.info.CFBundleShortVersionString
}

func (i *IPA) Identifier() string {
	return i.info.CFBundleIdentifier
}

func (i *IPA) Build() string {
	return i.info.CFBundleVersion
}

func (i *IPA) Channel() string {
	return i.info.Channel
}

func (i *IPA) Icon() image.Image {
	return i.icon
}

func (i *IPA) Size() int64 {
	return i.size
}
