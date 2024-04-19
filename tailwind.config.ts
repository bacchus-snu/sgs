import type { Config } from 'tailwindcss'
import tailwindForms from '@tailwindcss/forms'

export default {
	content: ['view/**/*.templ'],
	plugins: [tailwindForms],
} satisfies Config
