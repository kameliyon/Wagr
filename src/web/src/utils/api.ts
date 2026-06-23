const apiBase = import.meta.env.VITE_API_URL ?? ""

export const apiUrl = (path: string): string => `${apiBase}${path}`
