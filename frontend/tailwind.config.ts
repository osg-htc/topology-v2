import type { Config } from "tailwindcss";

// Palette modeled on the SWAMP app: a green "brand" primary and a "navy"
// sidebar/dark surface. Adjust to OSG branding as needed.
const config: Config = {
  content: ["./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        brand: {
          50: "#eef7ea",
          100: "#d6ecc9",
          200: "#b7dc9f",
          300: "#95cb72",
          400: "#79bd4f",
          500: "#5fa834",
          600: "#4d8b31",
          700: "#3d6e28",
          800: "#2f5420",
          900: "#25401a",
          950: "#12230c",
        },
        navy: {
          700: "#1e293b",
          800: "#172033",
          900: "#0f172a",
          950: "#080f1f",
        },
      },
    },
  },
  plugins: [],
};

export default config;
