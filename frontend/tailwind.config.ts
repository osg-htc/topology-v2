import type { Config } from "tailwindcss";

// Palette modeled on the SWAMP app: a green "brand" primary and a "navy"
// sidebar/dark surface. Adjust to OSG branding as needed.
const config: Config = {
  content: ["./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        // OSG-HTC-inspired green primary.
        brand: {
          50: "#e8f5ec",
          100: "#c7e7d1",
          200: "#9fd6b2",
          300: "#6fc08e",
          400: "#43a96c",
          500: "#1f9350",
          600: "#127a41",
          700: "#0f6335",
          800: "#0d4e2b",
          900: "#0b3d23",
          950: "#052414",
        },
        // Deep green for the sidebar / dark surfaces (replaces the old navy).
        navy: {
          700: "#123a26",
          800: "#0d2c1d",
          900: "#092015",
          950: "#04120b",
        },
        // Gold accent (OSG highlight).
        gold: {
          400: "#f4b740",
          500: "#e6a51e",
          600: "#c98a12",
        },
      },
    },
  },
  plugins: [],
};

export default config;
