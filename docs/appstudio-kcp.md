# Testing HAS on AppStudio-KCP Staging

## Prereqs

- Access to CPS
- A workspace created under your home workspace

## Testing HAS

1. Log on to CPS (either stable or unstable)

2. Navigate to your home workspace: `kubectl ws`

3. Create a workspace to test in: `kubectl ws create <workspace-name> --enter`

   - Alternatively, navigate to a preexisting workspace of your choosing with `kubectl ws <workspace-name>`

4. Create the necessary API Bindings for HAS:

   ```
   $ kubectl create -f hack/appstudio-kcp/api-binding.yaml && kubectl create -f hack/appstudio-kcp/has-binding.yaml
   ```

   This will create two API Bindings: One for the HAS API, the other to allow HAS to watch your workspace

5. If you need to test with SPI for private source repositories, then add the API Binding for SPI as well:

   ```
   $ kubectl create -f hack/appstudio-kcp/spi-binding.yaml
   ```
