import { shell } from "electron";

const ALLOWED_SCHEMES = ["http:", "https:", "mailto:"];

export function openExternalSafely(url: string): Promise<void> {
  try {
    const parsed = new URL(url);
    if (!ALLOWED_SCHEMES.includes(parsed.protocol)) {
      console.warn(`[security] blocked non-allowed scheme: ${parsed.protocol}`);
      return Promise.resolve();
    }
    return shell.openExternal(url);
  } catch (err) {
    console.warn(`[security] invalid URL blocked: ${url}`, err);
    return Promise.resolve();
  }
}