#!/bin/sh

# Disable core dump files for the shell and all child processes.
ulimit -c 0 || true

mkdir -p /data/lockdown
mkdir -p /data/PlumeImpactor
mkdir -p /data/PlumeImpactor/pairing_files
mkdir -p "$HOME/.config"
[ ! -e "$HOME/.config/PlumeImpactor" ] && ln -s /data/PlumeImpactor "$HOME/.config/PlumeImpactor"

# Remove core dumps created by previous image versions in the mounted data dir.
rm -f /data/core
for core_file in /data/core.[0-9]*; do
    case "$core_file" in
        *[!0-9]) continue ;;
    esac
    [ -f "$core_file" ] && rm -f "$core_file"
done

if [ -d "/keep/lib" ]; then
    rm -rf /data/PlumeImpactor/lib
    cp -rf /keep/lib /data/PlumeImpactor/lib
    rm -rf /keep/lib
fi

if [ -d "/keep/DeveloperDiskImages" ]; then
    rm -rf /data/DeveloperDiskImages
    cp -rf /keep/DeveloperDiskImages /data/DeveloperDiskImages
fi

if [ ! -f "/data/config.yaml" ]; then
    cp /keep/config.yaml /data/config.yaml
fi

/etc/init.d/usbmuxd start

exec /usr/bin/__APP_NAME__ server -p "${SERVICE_PORT:-80}" -c /data/config.yaml
