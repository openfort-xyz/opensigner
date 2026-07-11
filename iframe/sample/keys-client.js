// Simple browser-compatible Openfort client
class Openfort {
  constructor(
    publishableKey,
    accessToken = undefined,
    thirdPartyProvider = undefined,
    thirdPartyTokenType = undefined,
    hotStorageURL = "http://localhost:7052",
  ) {
    this._publishableKey = publishableKey;
    this._accessToken = accessToken;
    this.thirdPartyProvider = thirdPartyProvider;
    this.thirdPartyTokenType = thirdPartyTokenType;
    this._hotStorageURL = hotStorageURL;
    // Opaque MFA session token, set after a successful verification and sent
    // on subsequent gated requests via the X-MFA-Session header.
    this._mfaSessionToken = null;
  }

  setAccessToken(token) {
    this._accessToken = token;
  }

  _getAuthHeaders(requestId = null) {
    const headers = {
      "Content-Type": "application/json",
    };

    // Use JWT token for authentication if available
    if (this._accessToken) {
      headers["Authorization"] = `Bearer ${this._accessToken}`;
    } else if (this._publishableKey) {
      headers["Authorization"] = `Bearer ${this._publishableKey}`;
    }

    if (this.thirdPartyProvider && this.thirdPartyTokenType) {
      headers["X-Auth-Provider"] = this.thirdPartyProvider;
      headers["X-Token-Type"] = this.thirdPartyTokenType;
    }

    if (this._mfaSessionToken) {
      headers["X-MFA-Session"] = this._mfaSessionToken;
    }

    if (requestId) {
      headers["x-request-id"] = requestId;
    }

    return headers;
  }

  async _makeRequest(method, endpoint, data = null, requestId = null, isMfaRetry = false) {
    const url = `${this._hotStorageURL}${endpoint}`;
    const options = {
      method: method,
      headers: this._getAuthHeaders(requestId),
    };

    if (data && (method === "POST" || method === "PUT")) {
      options.body = JSON.stringify(data);
    }

    try {
      const response = await fetch(url, options);

      if (!response.ok) {
        const errorText = await response.text();
        const mfa = this._parseMfaRequired(response.status, errorText);
        if (mfa) {
          return await this._handleMfaRequired(mfa, isMfaRetry, () =>
            this._makeRequest(method, endpoint, data, requestId, true),
          );
        }
        console.error(`Request failed: ${response.status} - ${errorText}`);
        throw new Error(`Request failed: ${response.status} - ${errorText}`);
      }

      // Handle 204 No Content responses
      if (response.status === 204) {
        return null;
      }

      return await response.json();
    } catch (error) {
      console.error("Request error:", error);
      throw error;
    }
  }

  _parseMfaRequired(status, errorText) {
    if (status !== 403) return null;
    try {
      const body = JSON.parse(errorText);
      if (body && body.error === "mfa_required") return body;
    } catch (error) {
      // Not a JSON body: a plain 403, not an MFA gate.
    }
    return null;
  }

  // When hot storage answers 403 mfa_required, run the app-provided
  // onMfaRequired hook (which should complete a verification flow and
  // resolve truthy) and retry the original request once. Cancelling the
  // flow rejects the pending operation as if the user declined.
  async _handleMfaRequired(mfa, isMfaRetry, retry) {
    if (!isMfaRetry && this.onMfaRequired) {
      const verified = await this.onMfaRequired(mfa.methods || []);
      if (verified) {
        return await retry();
      }
      throw new Error("MFA verification declined by user");
    }
    const error = new Error("MFA verification required");
    error.mfaRequired = true;
    error.methods = mfa.methods || [];
    throw error;
  }

  async init(chainId, requestId = null) {
    return await this._makeRequest(
      "POST",
      "/v1/devices/init",
      { chainId },
      requestId,
    );
  }

  async register(chainId, address, share, requestId = null) {
    return await this._makeRequest(
      "POST",
      "/v1/devices/register",
      {
        chainId,
        address,
        share,
      },
      requestId,
    );
  }

  async disable(account, requestId = null) {
    return await this._makeRequest(
      "POST",
      `/v1/accounts/${account}/disable`,
      {},
      requestId,
    );
  }

  async exported(address, requestId = null) {
    return await this._makeRequest(
      "POST",
      "/v1/devices/exported",
      { address },
      requestId,
    );
  }

  async getDevice(deviceID, requestId = null) {
    return await this._makeRequest(
      "GET",
      `/v1/devices/${deviceID}`,
      null,
      requestId,
    );
  }

  // V2 API methods
  async listAccounts(chainType = null, requestId = null) {
    let endpoint = "/v2/accounts";
    if (chainType) {
      endpoint += `?chainType=${encodeURIComponent(chainType)}`;
    }
    return await this._makeRequest("GET", endpoint, null, requestId);
  }

  async getAccount(accountId, requestId = null) {
    return await this._makeRequest(
      "GET",
      `/v2/accounts/${accountId}`,
      null,
      requestId,
    );
  }

  async createAccount(accountData, requestId = null) {
    return await this._makeRequest(
      "POST",
      "/v2/accounts",
      accountData,
      requestId,
    );
  }

  async listSigners(accountId = null, signerType = null, requestId = null) {
    let endpoint = "/v2/signers";
    const params = new URLSearchParams();

    if (accountId) params.append("account", accountId);
    if (signerType) params.append("signerType", signerType);

    if (params.toString()) {
      endpoint += `?${params.toString()}`;
    }

    return await this._makeRequest("GET", endpoint, null, requestId);
  }

  async createSigner(signerData, requestId = null) {
    return await this._makeRequest(
      "POST",
      "/v2/signers",
      signerData,
      requestId,
    );
  }

  async getShamirDevice(deviceId, requestId = null) {
    return await this._makeRequest(
      "GET",
      `/v1/devices/${deviceId}`,
      null,
      requestId,
    );
  }

  async createShamirDevice(deviceData, requestId = null) {
    return await this._makeRequest(
      "POST",
      `/v1/devices/register`,
      deviceData,
      requestId,
    );
  }

  async exportShamirSigner(address, requestId = null) {
    return await this._makeRequest(
      "POST",
      `/v1/devices/exported`,
      { address },
      requestId,
    );
  }

  async importShare(shareData, requestId = null) {
    return await this._makeRequest(
      "POST",
      `/v2/accounts/import-share`,
      shareData,
      requestId,
    );
  }

  // MFA methods

  async listMfaMethods(requestId = null) {
    return await this._makeRequest("GET", "/v1/mfa/methods", null, requestId);
  }

  async enrollMfa(type, phoneNumber = null, requestId = null) {
    const payload = { type };
    if (phoneNumber) payload.phoneNumber = phoneNumber;
    return await this._makeRequest("POST", "/v1/mfa/enroll", payload, requestId);
  }

  // payload: { methodId, challengeId?, code? | credential? }
  async verifyMfaEnrollment(payload, requestId = null) {
    const result = await this._makeRequest("POST", "/v1/mfa/enroll/verify", payload, requestId);
    this._captureSessionToken(result);
    return result;
  }

  async createMfaChallenge(methodId, requestId = null) {
    return await this._makeRequest("POST", "/v1/mfa/challenges", { methodId }, requestId);
  }

  // payload: { challengeId, code? | credential? }
  async verifyMfa(payload, requestId = null) {
    const result = await this._makeRequest("POST", "/v1/mfa/verify", payload, requestId);
    this._captureSessionToken(result);
    return result;
  }

  _captureSessionToken(result) {
    if (result && result.sessionToken) {
      this._mfaSessionToken = result.sessionToken;
    }
  }

  async cancelMfaChallenge(challengeId, requestId = null) {
    return await this._makeRequest(
      "POST",
      `/v1/mfa/challenges/${encodeURIComponent(challengeId)}/cancel`,
      {},
      requestId,
    );
  }

  async unenrollMfa(methodId, requestId = null) {
    return await this._makeRequest(
      "DELETE",
      `/v1/mfa/methods/${encodeURIComponent(methodId)}`,
      null,
      requestId,
    );
  }
}

// Make it globally available
window.Openfort = Openfort;
