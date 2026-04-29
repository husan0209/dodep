import type { Config } from 'tailwindcss'

const config: Config = {
  content: [
    './src/pages/**/*.{js,ts,jsx,tsx,mdx}',
    './src/components/**/*.{js,ts,jsx,tsx,mdx}',
    './src/app/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  darkMode: 'class',
  theme: {
    extend: {
      fontFamily: {
        sans: ['var(--font-inter)', 'system-ui', 'sans-serif'],
        display: ['var(--font-montserrat)', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'ui-monospace', 'monospace'],
      },
      colors: {
        bg: {
          primary: 'rgb(var(--bg-primary))',
          secondary: 'rgb(var(--bg-secondary))',
          tertiary: 'rgb(var(--bg-tertiary))',
          elevated: 'rgb(var(--bg-elevated))',
          glass: 'rgb(var(--bg-glass))',
        },
        text: {
          primary: 'rgb(var(--text-primary))',
          secondary: 'rgb(var(--text-secondary))',
          muted: 'rgb(var(--text-muted))',
          disabled: 'rgb(var(--text-disabled))',
        },
        accent: {
          gold: 'rgb(var(--accent-gold))',
          'gold-dim': 'rgb(var(--accent-gold-dim))',
          cyan: 'rgb(var(--accent-cyan))',
          emerald: 'rgb(var(--accent-emerald))',
          rose: 'rgb(var(--accent-rose))',
          violet: 'rgb(var(--accent-violet))',
          orange: 'rgb(var(--accent-orange))',
          primary: 'rgb(var(--accent-primary))',
          success: 'rgb(var(--accent-success))',
          danger: 'rgb(var(--accent-danger))',
          warning: 'rgb(var(--accent-warning))',
          info: 'rgb(var(--accent-info))',
          live: 'rgb(var(--accent-live))',
        },
        border: {
          DEFAULT: 'rgb(var(--border))',
          light: 'rgb(var(--border-light))',
          glow: 'rgb(var(--border-glow))',
        },
        ring: 'rgb(var(--ring))',
      },
      borderRadius: {
        '2xl': '1rem',
        '3xl': '1.25rem',
        '4xl': '1.5rem',
      },
      boxShadow: {
        'glow-gold': '0 0 20px -5px rgb(var(--shadow-gold) / 0.3), 0 0 40px -10px rgb(var(--shadow-gold) / 0.15)',
        'glow-gold-sm': '0 0 12px -3px rgb(var(--shadow-gold) / 0.25)',
        'glow-cyan': '0 0 20px -5px rgb(var(--shadow-cyan) / 0.3)',
        'card': '0 4px 24px -6px rgb(0 0 0 / 0.4), inset 0 1px 0 0 rgb(255 255 255 / 0.03)',
        'card-hover': '0 8px 40px -8px rgb(0 0 0 / 0.5), inset 0 1px 0 0 rgb(255 255 255 / 0.05)',
      },
      animation: {
        'pulse-fast': 'pulse 1s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'fade-in': 'fadeIn 0.2s ease-out forwards',
        'slide-up': 'slideUp 0.3s cubic-bezier(0.16, 1, 0.3, 1) forwards',
        'scale-in': 'scaleIn 0.2s cubic-bezier(0.16, 1, 0.3, 1) forwards',
        'shimmer': 'shimmer 2s infinite linear',
        'live-pulse': 'livePulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'float': 'float 3s ease-in-out infinite',
        'glow-pulse': 'glowPulse 2s ease-in-out infinite',
      },
      keyframes: {
        fadeIn: {
          from: { opacity: '0' },
          to: { opacity: '1' },
        },
        slideUp: {
          from: { opacity: '0', transform: 'translateY(12px)' },
          to: { opacity: '1', transform: 'translateY(0)' },
        },
        scaleIn: {
          from: { opacity: '0', transform: 'scale(0.95)' },
          to: { opacity: '1', transform: 'scale(1)' },
        },
        shimmer: {
          '0%': { backgroundPosition: '200% 0' },
          '100%': { backgroundPosition: '-200% 0' },
        },
        livePulse: {
          '0%, 100%': { opacity: '1', transform: 'scale(1)' },
          '50%': { opacity: '0.5', transform: 'scale(0.92)' },
        },
        float: {
          '0%, 100%': { transform: 'translateY(0)' },
          '50%': { transform: 'translateY(-6px)' },
        },
        glowPulse: {
          '0%, 100%': { boxShadow: '0 0 20px -5px rgb(var(--shadow-gold) / 0.2)' },
          '50%': { boxShadow: '0 0 30px -3px rgb(var(--shadow-gold) / 0.4)' },
        },
      },
    },
  },
  plugins: [],
}

export default config
