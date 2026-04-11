/** @type {import('next').NextConfig} */
const isProd = process.env.NODE_ENV === 'production';
console.log('Current NODE_ENV:', process.env.NODE_ENV);
console.log('Is Production:', isProd);

const GATEWAY_ORIGIN = 'http://localhost:8080';
const MANAGEMENT_ORIGIN = 'http://localhost:8081';

const nextConfig = {
    // Only use static export for production builds
    ...(isProd ? {
        output: 'export',
        distDir: '../cmd/server/ui_assets',
    } : {}),
    images: { unoptimized: true },

    // Development proxy to avoid CORS and support remote development
    async rewrites() {
        return !isProd ? [
            {
                source: '/v1/:path*',
                destination: `${GATEWAY_ORIGIN}/v1/:path*`,
            },
            {
                source: '/health/:path*',
                destination: `${GATEWAY_ORIGIN}/health/:path*`,
            },
            {
                source: '/sandbox/:path*',
                destination: `${MANAGEMENT_ORIGIN}/sandbox/:path*`,
            },
            {
                source: '/key/:path*',
                destination: `${MANAGEMENT_ORIGIN}/key/:path*`,
            },
            {
                source: '/team/:path*',
                destination: `${MANAGEMENT_ORIGIN}/team/:path*`,
            },
            {
                source: '/user/:path*',
                destination: `${MANAGEMENT_ORIGIN}/user/:path*`,
            },
            {
                source: '/organization/:path*',
                destination: `${MANAGEMENT_ORIGIN}/organization/:path*`,
            },
            {
                source: '/spend/:path*',
                destination: `${MANAGEMENT_ORIGIN}/spend/:path*`,
            },
            {
                source: '/audit/:path*',
                destination: `${MANAGEMENT_ORIGIN}/audit/:path*`,
            },
            {
                source: '/global/:path*',
                destination: `${MANAGEMENT_ORIGIN}/global/:path*`,
            },
            {
                source: '/invitation/:path*',
                destination: `${MANAGEMENT_ORIGIN}/invitation/:path*`,
            },
            {
                source: '/control/:path*',
                destination: `${MANAGEMENT_ORIGIN}/control/:path*`,
            },
            {
                source: '/auth/:path*',
                destination: `${MANAGEMENT_ORIGIN}/auth/:path*`,
            },
            {
                source: '/metrics',
                destination: `${GATEWAY_ORIGIN}/metrics`,
            },
            {
                source: '/mcp/:path*',
                destination: `${GATEWAY_ORIGIN}/mcp/:path*`,
            },
        ] : [];
    },
};

export default nextConfig;
