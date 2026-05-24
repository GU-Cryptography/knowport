// Authentication token storage + API fetch wrapper.
// Single-flight refresh: concurrent 401s share one /api/auth/refresh call.

const ACCESS_KEY = "ailocalbase.accessToken";
const REFRESH_KEY = "ailocalbase.refreshToken";
const USER_KEY = "ailocalbase.user";

export interface AuthUser {
  id: string;
  username: string;
  created_at?: string;
}

export interface TokenBundle {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  user: AuthUser;
}

export const AUTH_EXPIRED_EVENT = "ailocalbase:auth-expired";

export function getAccessToken(): string | null {
  return localStorage.getItem(ACCESS_KEY);
}

export function getRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_KEY);
}

export function getStoredUser(): AuthUser | null {
  const raw = localStorage.getItem(USER_KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as AuthUser;
  } catch {
    return null;
  }
}

export function setTokens(bundle: TokenBundle): void {
  localStorage.setItem(ACCESS_KEY, bundle.access_token);
  localStorage.setItem(REFRESH_KEY, bundle.refresh_token);
  localStorage.setItem(USER_KEY, JSON.stringify(bundle.user));
  window.dispatchEvent(new CustomEvent("ailocalbase:auth-changed"));
}

export function clearTokens(): void {
  localStorage.removeItem(ACCESS_KEY);
  localStorage.removeItem(REFRESH_KEY);
  localStorage.removeItem(USER_KEY);
  window.dispatchEvent(new CustomEvent("ailocalbase:auth-changed"));
}

let refreshInflight: Promise<boolean> | null = null;

async function tryRefresh(): Promise<boolean> {
  if (refreshInflight) return refreshInflight;
  const refresh = getRefreshToken();
  if (!refresh) return false;
  refreshInflight = (async () => {
    try {
      const resp = await fetch("/api/auth/refresh", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: refresh }),
      });
      if (!resp.ok) return false;
      const bundle = (await resp.json()) as TokenBundle;
      setTokens(bundle);
      return true;
    } catch {
      return false;
    } finally {
      refreshInflight = null;
    }
  })();
  return refreshInflight;
}

// apiFetch wraps fetch with Authorization injection and 401 → refresh-once retry.
export async function apiFetch(
  input: RequestInfo,
  init?: RequestInit,
): Promise<Response> {
  const token = getAccessToken();
  const headers = new Headers(init?.headers || {});
  if (token && !headers.has("Authorization")) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  let response = await fetch(input, { ...init, headers });
  if (response.status !== 401) return response;

  const refreshed = await tryRefresh();
  if (!refreshed) {
    clearTokens();
    window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT));
    return response;
  }
  const newToken = getAccessToken();
  if (newToken) {
    headers.set("Authorization", `Bearer ${newToken}`);
  }
  return fetch(input, { ...init, headers });
}

// --- High-level auth API ---

export async function register(
  username: string,
  password: string,
): Promise<AuthUser> {
  const resp = await fetch("/api/auth/register", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({}));
    throw new Error(err.error || `register failed (${resp.status})`);
  }
  return resp.json();
}

export async function login(
  username: string,
  password: string,
): Promise<TokenBundle> {
  const resp = await fetch("/api/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({}));
    throw new Error(err.error || `login failed (${resp.status})`);
  }
  const bundle = (await resp.json()) as TokenBundle;
  setTokens(bundle);
  return bundle;
}

export async function logout(): Promise<void> {
  const refresh = getRefreshToken();
  try {
    await apiFetch("/api/auth/logout", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refresh }),
    });
  } catch {
    // Best-effort; clear local state regardless.
  }
  clearTokens();
}

export async function fetchMe(): Promise<AuthUser | null> {
  if (!getAccessToken()) return null;
  const resp = await apiFetch("/api/auth/me");
  if (!resp.ok) return null;
  return resp.json();
}
