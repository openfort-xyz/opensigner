import ChildProcess from 'node:child_process'
import Process from 'node:process'
import NodeFS from 'node:fs'
import NodePath from 'node:path'
import { defineConfig } from 'vocs';
import React from 'react';

import { sidebar } from './sidebar'

const commitSha =
  ChildProcess.execSync('git rev-parse --short HEAD').toString().trim() ||
  Process.env.VERCEL_GIT_COMMIT_SHA?.slice(0, 7)

if (
  Process.env.NODE_ENV === 'production' &&
  Process.env.VITE_VERCEL_ENV === 'production'
) {
  NodeFS.writeFileSync(
    NodePath.join(Process.cwd(), 'public', 'robots.txt'),
    ['User-agent: *', 'Allow: /'].join('\n'),
  )

}
export default defineConfig({
  rootDir: '.',
  description: 'Open-source and non-custodial and self-hostable private key management.',
  title: 'OpenSigner | Non-Custodial Wallet Key Management',
  logoUrl: {
    light: '/icons/open-signer-logo.svg',
    dark: '/icons/open-signer-logo.svg',
  },
  banner: 'If you like OpenSigner, give it a [star on GitHub ⭐](https://github.com/openfort-xyz/opensigner)!',
  iconUrl: "/icons/icon.svg",
  sidebar,
  aiCta: true,
  socials: [
    {
      icon: 'github',
      link: 'https://github.com/openfort-xyz/opensigner',
    },
    {
      icon: 'telegram',
      link: 'https://t.me/openfort',
    },
    {
      icon: 'x',
      link: 'https://x.com/openfort_hq',
    }
  ],
  theme: {
    accentColor: { light: '#004AAD', dark: '#004AAD' },
  },
  head() {
    return (
      <>
        <meta
          content="width=device-width, initial-scale=1, maximum-scale=1"
          name="viewport"
        />
        <meta content="https://opensigner.dev/og-image.png" property="og:image" />
        <meta content="image/png" property="og:image:type" />
        <meta content="1200" property="og:image:width" />
        <meta content="630" property="og:image:height" />
        <meta content={commitSha} name="x-app-version" />
        <meta
          content={
            process.env.VITE_VERCEL_ENV !== 'production'
              ? 'noindex, nofollow'
              : 'index, follow'
          }
          name="robots"
        />
      </>
    )
  },
})
