import { defineConfig } from "mygo-cli";

// The frontend lives in web/static (the web app's sources); the desktop
// window loads the embedded server's loopback URL, so the CLI only builds
// and packages the Go app. buildCommand copies the server's message
// catalogs from the repo root into the app, where main.go embeds them.
export default defineConfig({
  name: "atvloadly",
  identifier: "com.bitxeno.atvloadly",
  // CI passes the release tag (vX.Y.Z) or a nightly label; the DMG name and
  // Info.plist carry the bare version.
  version: process.env.VERSION ? process.env.VERSION.replace(/^v/, "") : "1.0.0",
  buildCommand: "npm run sync-locales",
  // Icon Composer icon; mygo build compiles it with actool (Xcode 26) and
  // falls back to icon.png's .icns when building without Xcode.
  macos: {
    icon: "AppIcon.icon",
    // A menu bar app: never in the Dock (main.go sets the accessory policy
    // at runtime for dev runs; the bundle says so from the start).
    infoPlist: { LSUIElement: true },
    // Release CI sets MACOS_SIGNING_IDENTITY; unset builds sign ad hoc.
    ...(process.env.MACOS_SIGNING_IDENTITY && {
      signingIdentity: process.env.MACOS_SIGNING_IDENTITY,
      notarize: {
        keychainProfile: process.env.MACOS_NOTARY_PROFILE || "notary",
      },
    }),
  },
  out: "build",
});
