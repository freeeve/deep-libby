import { defineConfig, presetIcons, presetWind, presetTypography } from "unocss"

export default defineConfig({
  presets: [
    presetWind(),
    presetIcons({
      collections: {
        logos: () =>
          import("@iconify-json/logos/icons.json").then((i) => i.default),
        uil: () =>
          import("@iconify-json/uil/icons.json").then((l) => l.default),
      },
    }),
    presetTypography(),
  ],
  theme: {
    colors: {
      "deep-blue": "#264653",
      teal: "#2A9D8F",
      yellow: "#E9C46A",
      orange: "#F4A261",
      coral: "#E76F51",
    },
  },
})
