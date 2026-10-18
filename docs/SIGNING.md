# Setting up code signing (one time)

Signed releases show the publisher's name in Windows' "protected your PC" window, and the warning fades as people install the app. This uses **Microsoft Azure Artifact Signing** (formerly Trusted Signing), about **$10/month**. Individual developers in the US and Canada can use it.

GitHub signs each release itself, logged in to Azure with a short-lived identity, so **no password or key is ever stored** in GitHub.

You'll need a few IDs from Azure along the way. Paste them into a Notepad file as you go; none of them are secret.

## 1. Azure account and subscription

1. Go to **portal.azure.com** and sign in or create an account (a credit card is needed even for paid-as-you-go).
2. Search for **Subscriptions** and make sure you have one, for example "Pay-As-You-Go". Copy its **Subscription ID**.
3. Search for **Resource providers** (inside your subscription) and **Register** `Microsoft.CodeSigning`.

## 2. Create the signing account

1. Search for **Artifact Signing accounts** (it may still be called **Trusted Signing accounts**) and press **Create**.
2. **Resource group:** create a new one, `warchest-signing`.
3. **Account name:** `warchest-signing`. Copy it as your **SIGNING_ACCOUNT**.
4. **Region:** East US (or the one nearest you). Note the region's endpoint, your **SIGNING_ENDPOINT**:
   - East US: `https://eus.codesigning.azure.net/`
   - West US: `https://wus.codesigning.azure.net/`
   - West Central US: `https://wcus.codesigning.azure.net/`
   - North Europe: `https://neu.codesigning.azure.net/`
   - West Europe: `https://weu.codesigning.azure.net/`
5. **Pricing:** Basic. Press **Review + create**, then **Create**.

## 3. Identity validation (Microsoft checks who you are)

1. Open the new account, then go to **Objects** > **Identity validations** > **New identity** > **Public**.
2. Choose **Individual** and fill in your legal name and address exactly as on your ID.
3. Follow the link to verify, usually with a photo ID and a selfie through Microsoft's partner. This can take from minutes to a few days. The publisher name players see is the name you validate here.

## 4. Certificate profile

Once the identity shows **Completed**:

1. Go to **Objects** > **Certificate profiles** > **Create** > **Public Trust**.
2. **Name:** `warchest-companion`. Copy it as your **SIGNING_PROFILE**.
3. Pick the identity you validated, then press **Create**.

## 5. Let GitHub sign (no password stored)

1. Search for **App registrations** > **New registration**. Name it `warchest-companion-github` and press **Register**.
2. On its overview page, copy:
   - **Application (client) ID** → your **AZURE_CLIENT_ID**
   - **Directory (tenant) ID** → your **AZURE_TENANT_ID**
3. Go to **Certificates & secrets** > **Federated credentials** > **Add credential**:
   - **Scenario:** GitHub Actions deploying Azure resources
   - **Organization:** `spifft`, **Repository:** `warchest-companion`
   - **Entity type:** Environment, **Environment name:** `release`
   - **Name:** `github-release`, then press **Add**.
4. Back in your signing account (step 2), go to **Access control (IAM)** > **Add role assignment**. Choose **Artifact Signing Certificate Profile Signer** (or **Trusted Signing Certificate Profile Signer**), then under **Members**, select `warchest-companion-github`, and assign it.

## 6. Tell GitHub the names

In **github.com/spifft/warchest-companion** go to **Settings** > **Environments** > **New environment**, and name it `release`.

Then go to **Settings** > **Secrets and variables** > **Actions** > **Variables** tab > **New repository variable**, once for each of these six:

| Name | Value |
|---|---|
| `AZURE_CLIENT_ID` | from step 5 |
| `AZURE_TENANT_ID` | from step 5 |
| `AZURE_SUBSCRIPTION_ID` | from step 1 |
| `SIGNING_ENDPOINT` | from step 2, for example `https://eus.codesigning.azure.net/` |
| `SIGNING_ACCOUNT` | `warchest-signing` |
| `SIGNING_PROFILE` | `warchest-companion` |

That's it. The next release (a `v…` tag) is signed automatically, and the build checks the signature before publishing.
