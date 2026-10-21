/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      fontFamily: {
        sans: ["IBM Plex Sans", "ui-sans-serif", "system-ui", "sans-serif"],
        mono: ["IBM Plex Mono", "ui-monospace", "monospace"],
      },
      colors: {
        canvas: "rgb(var(--hv-canvas) / <alpha-value>)",
        sidebar: "rgb(var(--hv-sidebar) / <alpha-value>)",
        panel: "rgb(var(--hv-panel) / <alpha-value>)",
        raised: "rgb(var(--hv-raised) / <alpha-value>)",
        line: "rgb(var(--hv-line) / <alpha-value>)",
        ink: {
          DEFAULT: "rgb(var(--hv-ink) / <alpha-value>)",
          dim: "rgb(var(--hv-ink-dim) / <alpha-value>)",
          faint: "rgb(var(--hv-ink-faint) / <alpha-value>)",
        },
        accent: {
          DEFAULT: "rgb(var(--hv-accent) / <alpha-value>)",
          dim: "rgb(var(--hv-accent-dim) / <alpha-value>)",
          ink: "rgb(var(--hv-accent-ink) / <alpha-value>)",
        },
        ok: "rgb(var(--hv-ok) / <alpha-value>)",
        warn: "rgb(var(--hv-warn) / <alpha-value>)",
        danger: "rgb(var(--hv-danger) / <alpha-value>)",
        meter: "rgb(var(--hv-meter) / <alpha-value>)",
      },
    },
  },
  plugins: [],
};
