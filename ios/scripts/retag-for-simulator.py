"""Marks Google's arm64 iPhone code as simulator code, so that the simulator on an Apple-silicon Mac can link it.

Usage: retag-for-simulator.py <input> <output>

The input is one architecture (arm64) of an ML Kit binary: a single object file or a static archive of them.
The machine code an iPhone runs and the code the simulator runs on an Apple-silicon Mac are the same; the
linker tells them apart only by the platform in each object's LC_BUILD_VERSION load command (2 for iOS, 7 for
the iOS simulator). This changes that one number in every object, in place, so nothing else in the file moves.
It stops with an error on anything it does not expect, rather than writing a file that half works.

Apple's `vtool` does the same for a single object file but cannot open archives, and one of Google's archives
holds several objects under the same name, so unpacking it to run `vtool` on each would lose some of them.
"""
import struct
import sys

MH_MAGIC_64 = 0xFEEDFACF
LC_BUILD_VERSION = 0x32
# The older load commands that name a platform. Replacing one would change the object's size, so they are
# refused; Google's current builds do not use them.
LC_VERSION_MIN = {0x24: "macOS", 0x25: "iOS", 0x2F: "tvOS", 0x30: "watchOS"}
PLATFORM_IOS = 2
PLATFORM_IOS_SIMULATOR = 7


def retag_object(data: bytearray, start: int, name: str) -> None:
    magic, _, _, _, ncmds, _, _, _ = struct.unpack_from("<IiiIIIII", data, start)
    if magic != MH_MAGIC_64:
        raise SystemExit(f"{name}: not a 64-bit Mach-O object")
    offset = start + 32  # past the 64-bit header
    tagged = 0
    for _ in range(ncmds):
        cmd, cmdsize = struct.unpack_from("<II", data, offset)
        if cmd in LC_VERSION_MIN:
            raise SystemExit(f"{name}: names its platform the old way ({LC_VERSION_MIN[cmd]}), not handled")
        if cmd == LC_BUILD_VERSION:
            (platform,) = struct.unpack_from("<I", data, offset + 8)
            if platform != PLATFORM_IOS:
                raise SystemExit(f"{name}: built for platform {platform}, not iOS")
            struct.pack_into("<I", data, offset + 8, PLATFORM_IOS_SIMULATOR)
            tagged += 1
        offset += cmdsize
    if tagged != 1:
        raise SystemExit(f"{name}: {tagged} build version commands, expected one")


def retag_archive(data: bytearray) -> int:
    """Retags every object in a BSD archive (the format Xcode's libtool writes). Returns how many."""
    offset = 8  # past "!<arch>\n"
    count = 0
    while offset < len(data):
        header = data[offset:offset + 60]
        if header[58:60] != b"`\n":
            raise SystemExit(f"archive member at byte {offset} has a damaged header")
        name = header[0:16].decode().strip()
        size = int(header[48:58].decode().strip())
        body = offset + 60
        # BSD archives keep long names after the header ("#1/<length>"), counted in the member's size.
        name_length = 0
        if name.startswith("#1/"):
            name_length = int(name[3:])
            name = data[body:body + name_length].decode().rstrip("\0")
        if not name.startswith("__.SYMDEF"):  # the archive's symbol table, which names no platform
            retag_object(data, body + name_length, name)
            count += 1
        offset = body + size
        offset += offset % 2  # members start on an even byte
    return count


def main() -> None:
    if len(sys.argv) != 3:
        raise SystemExit(__doc__)
    source, destination = sys.argv[1], sys.argv[2]
    with open(source, "rb") as file:
        data = bytearray(file.read())
    if data.startswith(b"!<arch>\n"):
        count = retag_archive(data)
    else:
        retag_object(data, 0, source)
        count = 1
    with open(destination, "wb") as file:
        file.write(data)
    print(f"retagged {count} object(s) for the simulator: {destination}")


if __name__ == "__main__":
    main()
