import { defineConfig } from "mygo-cli";

// The frontend lives in web/static (the web app's sources); the desktop
// window loads the embedded server's loopback URL, so the CLI only builds
// and packages the Go app. buildCommand copies the server's message
// catalogs from the repo root into the app, where main.go embeds them.
export default defineConfig({
  name: "atvloadly",
  identifier: "com.bitxeno.atvloadly",
  version: "1.0.0",
  buildCommand: "npm run sync-locales",
  // Icon Composer icon; mygo build compiles it with actool (Xcode 26) and
  // falls back to icon.png's .icns when building without Xcode.
  macos: {
    icon: "AppIcon.icon",
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
