"""Request one restart over the installer's existing paired connection."""

import asyncio
import plistlib
import sys
from pathlib import Path

from pymobiledevice3.lockdown import create_using_usbmux
from pymobiledevice3.remote.tunnel_service import RemotePairingTunnelService
from pymobiledevice3.remote.userspace_tunnel import TunnelProvider, UserspaceRsdTunnel
from pymobiledevice3.services.diagnostics import DiagnosticsService


class PlumePairing(RemotePairingTunnelService):
    """Read the host identity in Plume's plist format without copying keys to disk."""

    def __init__(self, udid, host, port, path):
        super().__init__(udid, host, int(port))
        with Path(path).open("rb") as stream:
            saved = plistlib.load(stream)
        self.record = {"private_key": saved["private_key"], "host_identifier": saved["identifier"]}

    @property
    def pair_record(self):
        return self.record


async def restart(device, udid):
    if device.udid != udid or not (device.product_type or "").startswith("AppleTV"):
        raise ValueError("The connected device does not match the target Apple TV")
    async with DiagnosticsService(device) as diagnostics:
        await diagnostics.restart()


async def main(connection, udid, *endpoint):
    if connection == "Lockdown":
        async with await create_using_usbmux(serial=udid, autopair=False) as device:
            await restart(device, udid)
    elif connection == "RPPairing":
        async def connect():
            service = PlumePairing(udid, *endpoint)
            await service.connect(autopair=False)
            return TunnelProvider(service)

        async with UserspaceRsdTunnel(provider_factory=connect) as device:
            await restart(device, udid)
    else:
        raise ValueError("Unsupported device connection")


if __name__ == "__main__":
    asyncio.run(main(*sys.argv[1:]))
