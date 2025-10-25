const BASE_URL = "";

async function fetchFromRetail(endpoint, method = "GET", body = null) {
  const url = `${BASE_URL}${endpoint}`;
  const options = {
    method,
    headers: {},
  };

  if (body !== null) {
    options.headers["Content-Type"] = "application/json";
    options.body = JSON.stringify(body);
  }

  try {
    const response = await fetch(url, options);

    if (!response.ok) {
      console.warn(
        `[RetailPlayerService] Request to ${endpoint} failed with status ${response.status} ${response.statusText}`
      );

      let errorDetail;
      try {
        errorDetail = await response.json();
      } catch (jsonError) {
        errorDetail = await response.text();
      }

      console.warn("Response detail:", errorDetail);
      throw new Error(`RetailPlayer API error: ${response.status}`);
    }

    const contentType = response.headers.get("content-type");
    if (contentType && contentType.includes("application/json")) {
      return response.json();
    }

    return null;
  } catch (error) {
    console.warn(`[RetailPlayerService] Network error while accessing ${endpoint}`, error);
    throw error;
  }
}

export function getDevices() {
  return fetchFromRetail(`/api/retailplayer/devices`);
}

export function getDevice(id) {
  return fetchFromRetail(`/api/retailplayer/devices/${id}`);
}

export function getDeviceStatus(id) {
  return fetchFromRetail(`/api/retailplayer/devices/${id}/status`);
}

export function postDeviceCommand(id, payload) {
  return fetchFromRetail(`/api/retailplayer/devices/${id}/command`, "POST", payload);
}

export default {
  getDevices,
  getDevice,
  getDeviceStatus,
  postDeviceCommand,
};

// Example usage for sanity checking device responses:
// getDevices().then(console.log);
