package asset

import "fmt"

type RenditionAttributeType uint32

const (
	kRenditionAttributeType_ThemeLook               = RenditionAttributeType(0)
	kRenditionAttributeType_Element                 = RenditionAttributeType(1)
	kRenditionAttributeType_Part                    = RenditionAttributeType(2)
	kRenditionAttributeType_Size                    = RenditionAttributeType(3)
	kRenditionAttributeType_Direction               = RenditionAttributeType(4)
	kRenditionAttributeType_placeholder             = RenditionAttributeType(5)
	kRenditionAttributeType_Value                   = RenditionAttributeType(6)
	kRenditionAttributeType_ThemeAppearance         = RenditionAttributeType(7)
	kRenditionAttributeType_Dimension1              = RenditionAttributeType(8)
	kRenditionAttributeType_Dimension2              = RenditionAttributeType(9)
	kRenditionAttributeType_State                   = RenditionAttributeType(10)
	kRenditionAttributeType_Layer                   = RenditionAttributeType(11)
	kRenditionAttributeType_Scale                   = RenditionAttributeType(12)
	kRenditionAttributeType_Unknown13               = RenditionAttributeType(13)
	kRenditionAttributeType_PresentationState       = RenditionAttributeType(14)
	kRenditionAttributeType_Idiom                   = RenditionAttributeType(15)
	kRenditionAttributeType_Subtype                 = RenditionAttributeType(16)
	kRenditionAttributeType_Identifier              = RenditionAttributeType(17)
	kRenditionAttributeType_PreviousValue           = RenditionAttributeType(18)
	kRenditionAttributeType_PreviousState           = RenditionAttributeType(19)
	kRenditionAttributeType_HorizontalSizeClass     = RenditionAttributeType(20)
	kRenditionAttributeType_VerticalSizeClass       = RenditionAttributeType(21)
	kRenditionAttributeType_MemoryLevelClass        = RenditionAttributeType(22)
	kRenditionAttributeType_GraphicsFeatureSetClass = RenditionAttributeType(23)
	kRenditionAttributeType_DisplayGamut            = RenditionAttributeType(24)
	kRenditionAttributeType_DeploymentTarget        = RenditionAttributeType(25)
)

func (t RenditionAttributeType) String() string {
	switch t {
	case kRenditionAttributeType_ThemeLook:
		return "Theme Look"
	case kRenditionAttributeType_Element:
		return "Element"
	case kRenditionAttributeType_Part:
		return "Part"
	case kRenditionAttributeType_Size:
		return "Size"
	case kRenditionAttributeType_Direction:
		return "Direction"
	case kRenditionAttributeType_placeholder:
		return "placeholder"
	case kRenditionAttributeType_Value:
		return "Value"
	case kRenditionAttributeType_ThemeAppearance:
		return "Theme Appearance"
	case kRenditionAttributeType_Dimension1:
		return "Dimension 1"
	case kRenditionAttributeType_Dimension2:
		return "Dimension 2"
	case kRenditionAttributeType_State:
		return "State"
	case kRenditionAttributeType_Layer:
		return "Layer"
	case kRenditionAttributeType_Scale:
		return "Scale"
	case kRenditionAttributeType_PresentationState:
		return "Presentation State"
	case kRenditionAttributeType_Idiom:
		return "Idiom"
	case kRenditionAttributeType_Subtype:
		return "Subtype"
	case kRenditionAttributeType_Identifier:
		return "Identifier"
	case kRenditionAttributeType_PreviousValue:
		return "Previous Value"
	case kRenditionAttributeType_PreviousState:
		return "Previous State"
	case kRenditionAttributeType_HorizontalSizeClass:
		return "Horizontal Size Class"
	case kRenditionAttributeType_VerticalSizeClass:
		return "Vertical Size Class"
	case kRenditionAttributeType_MemoryLevelClass:
		return "Memory Level Class"
	case kRenditionAttributeType_GraphicsFeatureSetClass:
		return "Graphics Feature Set Class"
	case kRenditionAttributeType_DisplayGamut:
		return "Display Gamut"
	case kRenditionAttributeType_DeploymentTarget:
		return "Deployment Target"
	default:
		return fmt.Sprintf("Unknown %d", t)
	}
}
