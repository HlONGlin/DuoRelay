# About-Page Update Checks and One-Click Updates

[简体中文](app-update.md) | English

Under **Settings → About**, choose **Check for updates**. DuoRelay queries the latest GitHub stable release and selects an asset for the current platform and full/lite variant. Supported assets are Linux amd64/arm64 and Windows amd64, with full and lite packages on each. If no stable release is published, the platform asset is missing, or the install directory is not writable, the page explains the reason and disables installation.

Review the version and release notes, then choose **Update and restart**. DuoRelay downloads and verifies the asset size and GitHub SHA-256 digest when provided, checks archive paths and file types, replaces the executable and bundled kernels, and restarts. The updater does not replace the database, nodes or settings. Keep the application open while it prepares the update.

## Installation constraints

- **Windows:** The current user must be able to write to the DuoRelay program directory. Use an appropriately privileged deployment method for protected install locations.
- **Writable Linux installs:** In-place updates are available when the program directory is writable. The running process replaces its executable and starts the new version in place.
- **Default Linux systemd service:** `/opt/duorelay` is managed by root and protected by the service sandbox. The About page can still check versions, but one-click installation is disabled. Stop the service, back up data and reinstall the target release using the [Linux deployment guide](linux-deployment.en.md). Updates do not elevate service privileges or relax system-directory protection.
- **Docker:** Update by pulling or rebuilding the target image; do not self-update the binary inside a container. See [Docker deployment](docker-deployment.en.md).

Automatic updates require a GitHub stable release with an asset matching the current package naming. If checking reports no release or fails, try again later or follow the platform deployment guide to upgrade manually.
