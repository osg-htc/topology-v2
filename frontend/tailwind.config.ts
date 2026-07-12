import type { Config } from "tailwindcss";

// Palette modeled on the SWAMP app: a green "brand" primary and a "navy"
// sidebar/dark surface. Adjust to OSG branding as needed.
const config: Config = {
  content: ["./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        // OSG-HTC brand: primary is the bronze/tan #b67b3c (from the site's
        // Bootstrap $primary), used for buttons, links, and active nav.
        brand: {
          50: "#f8f1e9",
          100: "#eeddc8",
          200: "#e0c29d",
          300: "#d0a471",
          400: "#c48c50",
          500: "#b67b3c",
          600: "#9c6733",
          700: "#7e522a",
          800: "#603f22",
          900: "#472f1b",
          950: "#29190d",
        },
        // OSG dark navy for the sidebar / dark surfaces (#203050 → #0b1725).
        navy: {
          700: "#24406b",
          800: "#1a3050",
          900: "#122238",
          950: "#0b1725",
        },
        // OSG gold (Bootstrap $secondary) accent.
        gold: {
          400: "#f6c34f",
          500: "#f4b627",
          600: "#d99e15",
        },
        // OSG info blue (Bootstrap $info), for occasional accents/links.
        info: {
          500: "#2b77f1",
          600: "#1f61cf",
        },
      },
    },
  },
  plugins: [],
};

export default config;
