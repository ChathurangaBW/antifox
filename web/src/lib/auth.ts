export const TOKEN_KEY = "antifox_token";
// Auto-logout disabled: keep the cookie for ~10 years (session ends on explicit logout).
export const COOKIE_MAX_AGE = 10 * 365 * 24 * 60 * 60;

export interface CurrentUser {
  id: string;
  name: string;
  username: string;
  email: string;
  avatar: string;
  role: string;
}

export const auth = {
  getToken(): string | null {
    if (typeof window === "undefined") return null;
    // Mock demo:No real login, return a fake token Let the guards walk and enter the main interface.
    return localStorage.getItem(TOKEN_KEY) ?? (process.env.NEXT_PUBLIC_MOCK === "1" ? "mock-demo" : null);
  },

  setToken(token: string): void {
    localStorage.setItem(TOKEN_KEY, token);
    // Sync cookie,For Next.js middleware Service Read
    document.cookie = `${TOKEN_KEY}=${encodeURIComponent(token)}; path=/; max-age=${COOKIE_MAX_AGE}; SameSite=Lax`;
  },

  clearToken(): void {
    localStorage.removeItem(TOKEN_KEY);
    document.cookie = `${TOKEN_KEY}=; path=/; max-age=0`;
  },

  // From JWT payload Parsing the current user, only for display, not for signature authentication.
  // sub = User id,usr = Username (multi-user) RBAC).
  getCurrentUser(): CurrentUser | null {
    const token = this.getToken();
    if (!token) return null;
    try {
      const parts = token.split(".");
      if (parts.length !== 3) return null;
      // base64url → base64
      const payload = JSON.parse(atob(parts[1].replace(/-/g, "+").replace(/_/g, "/")));
      const username: string = payload.usr ?? payload.sub ?? "admin";
      const id: string = payload.sub ?? "";
      return { id, name: username, username, email: "", avatar: "", role: "operator" };
    } catch {
      return null;
    }
  },
};
