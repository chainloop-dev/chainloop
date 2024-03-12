// @ts-check
// Note: type annotations allow type checking and IDEs autocompletion

const lightCodeTheme = require("prism-react-renderer/themes/github");
const darkCodeTheme = require("prism-react-renderer/themes/dracula");

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: "Chainloop documentation",
  tagline: "The Software Supply Chain Attestation Solution that makes sense",
  url: "https://docs.chainloop.dev",
  baseUrl: "/",
  onBrokenLinks: "throw",
  onBrokenMarkdownLinks: "warn",
  favicon: "img/logo-clear.svg",

  // Even if you don't use internalization, you can use this field to set useful
  // metadata like html lang. For example, if your site is Chinese, you may want
  // to replace "en" with "zh-Hans".
  i18n: {
    defaultLocale: "en",
    locales: ["en"],
  },

  plugins: [
    [
      "@docusaurus/plugin-ideal-image",
      {
        disableInDev: false,
      },
    ],
    // Download remote content
    // Plugin overview guide, to update it run
    // yarn run docusaurus download-remote-plugins-overview
    [
      "docusaurus-plugin-remote-content",
      {
        name: "plugins-overview",
        // DO NOT automatically download the file
        noRuntimeDownloads: true,
        // Keep the downloaded files
        performCleanup: false,
        sourceBaseUrl:
          "https://raw.githubusercontent.com/chainloop-dev/chainloop/main/app/controlplane/plugins/",
        outDir: "docs/integrations/development", // the base directory to output to.
        documents: ["README.md"], // the file names to download
        modifyContent: (filename, content) => {
          if (filename.includes("README")) {
            return {
              content: `---
title: Plugins Overview
image: /img/fanout-sdk.png
---

${content.replaceAll("../../../docs/img/", "/img/")}`,
              filename: "overview.md",
            };
          }

          return undefined;
        },
      },
    ],
    // Download the associated images, to update it
    // yarn run docusaurus download-remote-plugins-overview-img
    [
      "docusaurus-plugin-remote-content",
      {
        name: "plugins-overview-img",
        noRuntimeDownloads: true,
        performCleanup: false,
        sourceBaseUrl:
          "https://raw.githubusercontent.com/chainloop-dev/chainloop/main/docs/img",
        outDir: "static/img",
        documents: [
          "fanout.png",
          "fanout-sdk.png",
          "fanout-execute-materials.png",
          "fanout-execute.png",
        ],
        requestConfig: { responseType: "arraybuffer" },
      },
    ],
    // Integrations index remote content
    // yarn run docusaurus download-remote-integrations-index
    [
      "docusaurus-plugin-remote-content",
      {
        name: "integrations-index",
        noRuntimeDownloads: true,
        performCleanup: false,
        sourceBaseUrl:
          "https://raw.githubusercontent.com/chainloop-dev/chainloop/main/devel",
        outDir: "docs/integrations",
        documents: ["integrations.md"],
        modifyContent: (filename, content) => {
          if (filename.includes("integrations.md")) {
            return {
              content: `---
title: Integrations
image: /img/fanout.png
---

${content.replaceAll("./img/fanout.png", "/img/fanout.png")}`,
            };
          }

          return undefined;
        },
      },
    ],
    // Helm Chart readme
    // yarn run docusaurus download-remote-deployment-readme
    [
      "docusaurus-plugin-remote-content",
      {
        name: "deployment-readme",
        noRuntimeDownloads: true,
        performCleanup: false,
        sourceBaseUrl:
          "https://raw.githubusercontent.com/chainloop-dev/chainloop/main/deployment/chainloop",
        outDir: "docs/guides/deployment/k8s/", // the base directory to output to.
        documents: ["README.md"], // the file names to download
        modifyContent: (filename, content) => {
          if (filename.includes("README")) {
            return {
              content: `---
title: Deploy on Kubernetes
image: ./deployment.png
---
import Image from "@theme/IdealImage";

${content
  .replaceAll(
    "![Deployment](../../docs/img/deployment.png)",
    '<Image img={require("./deployment.png")} className="light-mode-only" /> <Image img={require("./deployment-dark.png")} className="dark-mode-only" />'
  )
  .replaceAll(
    "![Deployment](../../docs/img/deployment-dev.png)",
    '<Image img={require("./deployment-dev.png")} className="light-mode-only" /> <Image img={require("./deployment-dev-dark.png")} className="dark-mode-only" />'
  )} `,
              filename: "k8s.mdx",
            };
          }

          return undefined;
        },
      },
    ],
  ],

  presets: [
    [
      "classic",
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        gtag: {
          trackingID: "G-G4XVJP8LEP",
        },
        docs: {
          sidebarPath: require.resolve("./sidebars.js"),
          sidebarCollapsed: false,
          // Docs only mode
          routeBasePath: "/",
          editUrl: "https://github.com/chainloop-dev/chainloop/blob/main/docs",
        },
        blog: {
          showReadingTime: true,
          blogTitle: "Chainloop blog",
          blogDescription: "Chainloop blog",
        },
        theme: {
          customCss: require.resolve("./src/css/custom.css"),
        },
      }),
    ],
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      algolia: {
        appId: "80E4FPODSO",
        // Public API key: it is safe to commit it
        apiKey: "2757faaa5b66256f54f2f84fe4005a4a",
        indexName: "chainloop",
        // Optional: see doc section below
        contextualSearch: true,
      },
      colorMode: {
        defaultMode: "dark",
        disableSwitch: false,
        respectPrefersColorScheme: false,
      },
      image: "img/logo.svg",
      navbar: {
        title: "Chainloop documentation",
        logo: {
          alt: "Chainloop Logo",
          srcDark: "img/logo-clear.svg",
          src: "img/logo.svg",
        },
        items: [
          {
            href: "https://chainloop.dev/blog",
            label: "Blog",
            position: "right",
          },
          {
            href: "https://github.com/chainloop-dev/chainloop",
            position: "right",
            className: "header-github-link",
            "aria-label": "GitHub repository",
          },
          {
            href: "https://discord.gg/Sfw3HnRt",
            position: "right",
            className: "header-discord-link",
            "aria-label": "Discord Server",
          },
        ],
      },
      footer: {
        style: "dark",
        links: [
          {
            label: "Main site",
            href: "https://chainloop.dev",
          },
          {
            label: "Contact",
            href: "https://chainloop.dev/contact",
          },
          {
            label: "Blog",
            href: "https://chainloop.dev/blog",
          },
          {
            label: "GitHub",
            href: "https://github.com/chainloop-dev",
          },
        ],
      },
      prism: {
        theme: lightCodeTheme,
        darkTheme: darkCodeTheme,
        additionalLanguages: ["log", "cue"],
      },
      metadata: [
        {
          name: "keywords",
          content:
            "software supply chain, security, attestation, slsa, sigstore, in-toto",
        },
      ],
    }),
  scripts: [],
};

module.exports = config;
