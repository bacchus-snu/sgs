import type { Config } from 'postcss-load-config'
import tailwindcss from '@tailwindcss/postcss'

export default {
	plugins: [tailwindcss()],
} satisfies Config
