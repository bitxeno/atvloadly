package web

import (
	"fmt"
	"io"
	"net/http"

	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/service"
	"github.com/bitxeno/atvloadly/internal/signing"
	"github.com/bitxeno/atvloadly/internal/signing/appcheck"
	"github.com/bitxeno/atvloadly/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// signingCheckResult is the payload of the compatibility check route.
type signingCheckResult struct {
	Plan     *appcheck.Plan `json:"plan"`
	Blocking bool           `json:"blocking"`
}

// registerSigningRoutes registers the external signing identity API under /api/signing.
func registerSigningRoutes(api fiber.Router) {
	api.Get("/signing/identities", func(c *fiber.Ctx) error {
		views, err := service.ListSigningIdentityViews()
		if err != nil {
			return c.Status(http.StatusOK).JSON(apiSigningError(err))
		}
		return c.Status(http.StatusOK).JSON(apiSuccess(views))
	})

	api.Post("/signing/identities/import", func(c *fiber.Ctx) error {
		p12, err := readSigningUpload(c, "p12", "P12", signing.MaxP12Size)
		if err != nil {
			return c.Status(http.StatusOK).JSON(apiSigningError(err))
		}
		profile, err := readSigningUpload(c, "profile", "provisioning profile", signing.MaxProfileSize)
		if err != nil {
			return c.Status(http.StatusOK).JSON(apiSigningError(err))
		}

		view, issues, err := service.ImportSigningIdentity(signing.ImportRequest{
			Name:     c.FormValue("name"),
			P12:      p12,
			Password: c.FormValue("password"),
			Profile:  profile,
		})
		if err != nil {
			log.Infof("Signing identity import refused: %s", signing.CodeOf(err))
			return c.Status(http.StatusOK).JSON(apiSigningError(err))
		}
		log.Infof("Signing identity %d imported (team %s, %d finding(s))", view.ID, view.TeamID, len(issues))
		return c.Status(http.StatusOK).JSON(apiSuccess(view))
	})

	api.Post("/signing/identities/:id/profile", func(c *fiber.Ctx) error {
		id := uint(utils.MustParseInt(c.Params("id")))
		profile, err := readSigningUpload(c, "profile", "provisioning profile", signing.MaxProfileSize)
		if err != nil {
			return c.Status(http.StatusOK).JSON(apiSigningError(err))
		}

		view, _, err := service.ReplaceSigningIdentityProfile(id, profile)
		if err != nil {
			return c.Status(http.StatusOK).JSON(apiSigningError(err))
		}
		log.Infof("Signing identity %d profile replaced (revision %d)", view.ID, view.Revision)
		return c.Status(http.StatusOK).JSON(apiSuccess(view))
	})

	api.Post("/signing/identities/:id/delete", func(c *fiber.Ctx) error {
		id := uint(utils.MustParseInt(c.Params("id")))
		var req struct {
			Force bool `json:"force"`
		}
		if len(c.Body()) > 0 {
			if err := c.BodyParser(&req); err != nil {
				return c.Status(http.StatusOK).JSON(apiError("Invalid argument"))
			}
		}

		if err := service.DeleteSigningIdentity(id, req.Force); err != nil {
			result := apiSigningError(err)
			if data, ok := result.Data.(SigningErrorData); ok && data.Code == signing.CodeIdentityReferenced {
				if count, countErr := service.CountAppsBySigningIdentity(id); countErr == nil {
					data.AppCount = count
					result.Data = data
				}
			}
			return c.Status(http.StatusOK).JSON(result)
		}
		log.Infof("Signing identity %d deleted", id)
		return c.Status(http.StatusOK).JSON(apiSuccess(true))
	})

	api.Post("/signing/identities/:id/check", func(c *fiber.Ctx) error {
		id := uint(utils.MustParseInt(c.Params("id")))
		var req struct {
			IpaPath                  string `json:"ipa_path"`
			UDID                     string `json:"udid"`
			RemoveExtensions         bool   `json:"remove_extensions"`
			AllowMissingEntitlements bool   `json:"allow_missing_entitlements"`
			CustomIdentifier         string `json:"custom_identifier"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(http.StatusOK).JSON(apiError("Invalid argument"))
		}

		plan, err := service.CheckExternalInstall(service.ExternalCheckRequest{
			IdentityID:               id,
			IpaPath:                  req.IpaPath,
			UDID:                     req.UDID,
			RemoveExtensions:         req.RemoveExtensions,
			AllowMissingEntitlements: req.AllowMissingEntitlements,
			CustomIdentifier:         req.CustomIdentifier,
		})
		if err != nil {
			return c.Status(http.StatusOK).JSON(apiSigningError(err))
		}
		return c.Status(http.StatusOK).JSON(apiSuccess(signingCheckResult{Plan: plan, Blocking: plan.Blocking()}))
	})
}

// readSigningUpload reads the multipart file field, refusing files larger than
// limit before opening them. A missing file is a plain error; an oversized one
// is a *signing.Error with CodeUploadTooLarge.
func readSigningUpload(c *fiber.Ctx, field, label string, limit int64) ([]byte, error) {
	header, err := c.FormFile(field)
	if err != nil {
		return nil, fmt.Errorf("no %s file uploaded", label)
	}
	if header.Size > limit {
		return nil, uploadTooLargeError(label, limit)
	}

	file, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to read the uploaded %s file: %w", label, err)
	}
	defer func() {
		_ = file.Close()
	}()

	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read the uploaded %s file: %w", label, err)
	}
	if int64(len(data)) > limit {
		return nil, uploadTooLargeError(label, limit)
	}
	return data, nil
}

func uploadTooLargeError(label string, limit int64) error {
	return signing.Errorf(signing.ClassIdentity, signing.CodeUploadTooLarge,
		"the %s file exceeds the maximum size of %d bytes", label, limit)
}
