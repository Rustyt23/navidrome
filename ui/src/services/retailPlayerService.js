const BASE_URL = "https://rpp.jareddietch.com/broad/api/v1";
const API_KEY = "f3894t28-aghj-cv50-453e-9dfr1s9h73s5";
const ORG_ID = "1aa59b04-5365-4efe-afb3-deb23c414add";

async function fetchFromRetail(endpoint, method = "GET", body = null) {
  const url = `${BASE_URL}${endpoint}`;
  const options = {
    method,
    headers: {
      "x-retailplayer-apikey": API_KEY,
      "Content-Type": "application/json",
    },
  };

  if (body !== null) {
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
  return fetchFromRetail(`/orgs/${ORG_ID}/devices`);
}

export function getDeviceStatus(id) {
  return fetchFromRetail(`/orgs/${ORG_ID}/devices/${id}/status`);
}

export function postDeviceCommand(id, payload) {
  return fetchFromRetail(`/orgs/${ORG_ID}/devices/${id}/command`, "POST", payload);
}

export default {
  getDevices,
  getDeviceStatus,
  postDeviceCommand,
};

// Example usage for sanity checking device responses:
// getDevices().then(console.log);
