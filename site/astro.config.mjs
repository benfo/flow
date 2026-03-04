// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// https://astro.build/config
export default defineConfig({
	site: 'https://benfo.github.io',
	base: '/flow',
	integrations: [
		starlight({
			title: 'flow',
			description: 'A terminal-based developer task dashboard',
			social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/benfo/flow' }],
			sidebar: [
				{
					label: 'Getting Started',
					items: [
						{ label: 'Introduction', slug: 'getting-started/introduction' },
						{ label: 'Installation', slug: 'getting-started/installation' },
					],
				},
				{
					label: 'Guides',
					items: [
						{ label: 'Git Integration', slug: 'guides/git-integration' },
						{ label: 'Providers', slug: 'guides/providers' },
					],
				},
				{
					label: 'Reference',
					items: [
						{ label: 'Keyboard Shortcuts', slug: 'reference/keyboard-shortcuts' },
					],
				},
			],
		}),
	],
});
