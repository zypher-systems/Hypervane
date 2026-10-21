/** Join the PVE token owner and secret into the token the API stores. */
export function composePVEToken(user: string, secret: string): string {
  const u = user.trim();
  const s = secret.trim().replace(/^PVEAPIToken=/, "");
  if (s.includes("=")) return s;
  if (u.includes("=")) return u;
  if (!u || !s) return "";
  return `${u}=${s}`;
}

export function pveMigrateToken(user: string, secret: string): string {
  const token = composePVEToken(user, secret);
  if (!token) return "";
  return token.startsWith("PVEAPIToken=") ? token : `PVEAPIToken=${token}`;
}
