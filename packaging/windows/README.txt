regattaClock - portable Windows build
=====================================

regattaClock collects and publishes race times at rowing regattas. It is run at
the finish line. See https://github.com/comagnaw/regattaClock for the operator
workflow and screenshots.

Running
-------
Unzip anywhere and double-click regattaClock.exe. No install required.
Configuration is stored in the Windows registry (Fyne Preferences); nothing is
written next to the executable.

`regattaClock.exe -v` prints the build's version, commit, and build date.

Verifying this download
----------------------
Every release has a SHA256SUMS file. Check this zip against it:

    Get-FileHash .\regattaClock-<version>-windows-amd64-portable.zip -Algorithm SHA256

and compare the hash to the matching line in SHA256SUMS. Releases also carry a
GitHub build-provenance attestation:

    gh attestation verify .\regattaClock-<version>-windows-amd64-portable.zip --repo comagnaw/regattaClock

Unsigned build
--------------
This build is not yet code-signed, so Windows SmartScreen may warn
("Windows protected your PC" -> More info -> Run anyway) and the publisher shows
as unknown. Verify the hash and the attestation above before running.
