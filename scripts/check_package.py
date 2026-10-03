#!/usr/bin/env python3
import sys
import zipfile
from pathlib import Path

def check_package(zip_path_str: str, expected_id: str, expected_version: str):
    zip_path = Path(zip_path_str)
    if not zip_path.exists():
        print(f"Error: package file does not exist: {zip_path}")
        return 1

    filename = zip_path.name
    # Expect format: <id>_<version>_<goos>_<goarch>.zip
    parts = filename[:-4].split("_")
    if len(parts) != 4:
        print(f"Error: archive name {filename} does not match expected format <id>_<version>_<goos>_<goarch>.zip")
        return 1

    pkg_id, pkg_version, goos, goarch = parts
    if pkg_id != expected_id:
        print(f"Error: package ID {pkg_id} does not match expected {expected_id}")
        return 1
    if pkg_version != expected_version:
        print(f"Error: package version {pkg_version} does not match expected {expected_version}")
        return 1

    ext_map = {
        "darwin": ".dylib",
        "linux": ".so",
        "windows": ".dll",
    }
    expected_ext = ext_map.get(goos)
    if not expected_ext:
        print(f"Error: unknown GOOS {goos}")
        return 1

    expected_plain_lib = f"{expected_id}{expected_ext}"
    expected_versioned_lib = f"{expected_id}-v{expected_version}{expected_ext}"

    with zipfile.ZipFile(zip_path, 'r') as z:
        entries = z.namelist()
        if not entries:
            print("Error: zip archive is empty")
            return 1

        print(f"Archive entries in {filename}: {entries}")

        # Check for dynamic libraries
        dyn_libs = [e for e in entries if e.endswith(('.so', '.dylib', '.dll'))]
        if len(dyn_libs) != 1:
            print(f"Error: archive must contain exactly one dynamic library, found {len(dyn_libs)}: {dyn_libs}")
            return 1

        target_lib = dyn_libs[0]
        # Target dynamic library must be at zip root
        if "/" in target_lib or "\\" in target_lib:
            print(f"Error: target dynamic library {target_lib} must be at the root of the archive")
            return 1

        if target_lib != expected_plain_lib and target_lib != expected_versioned_lib:
            print(f"Error: dynamic library filename {target_lib} must be {expected_plain_lib} or {expected_versioned_lib}")
            return 1

        # Check that there are no nested directories
        for entry in entries:
            if "/" in entry.rstrip("/") or "\\" in entry.rstrip("\\"):
                print(f"Error: nested path {entry} found in zip archive")
                return 1

    print(f"✓ Package {filename} verified successfully.")
    return 0

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: check_package.py <path-to-zip> [expected_id] [expected_version]")
        sys.exit(1)

    zip_file = sys.argv[1]
    exp_id = sys.argv[2] if len(sys.argv) > 2 else "cpa-live-voice"
    exp_ver = sys.argv[3] if len(sys.argv) > 3 else "0.1.0"

    sys.exit(check_package(zip_file, exp_id, exp_ver))
