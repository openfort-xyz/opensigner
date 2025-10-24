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

    if (requestId) {
      headers["x-request-id"] = requestId;
    }

    return headers;
  }

  async _makeRequest(method, endpoint, data = null, requestId = null) {
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

  async switchChain(chainId, deviceId, requestId = null) {
    return await this._makeRequest(
      "POST",
      "/v1/devices/switch-chain",
      {
        chainId,
        deviceId,
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
}

// Make it globally available
window.Openfort = Openfort;
