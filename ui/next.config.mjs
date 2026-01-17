/** @type {import('next').NextConfig} */
const isProd = process.env.NODE_ENV === 'production';
console.log('Current NODE_ENV:', process.env.NODE_ENV);
console.log('Is Production:', isProd);

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
                destination: 'http://localhost:8080/v1/:path*',
            },
            {
                source: '/health/:path*',
                destination: 'http://localhost:8080/health/:path*',
            },
            {
                source: '/key/:path*',
                destination: 'http://localhost:8081/key/:path*',
            },
            {
                source: '/team/:path*',
                destination: 'http://localhost:8081/team/:path*',
            },
            {
                source: '/user/:path*',
                destination: 'http://localhost:8081/user/:path*',
            },
            {
                source: '/organization/:path*',
                destination: 'http://localhost:8081/organization/:path*',
            },
            {
                source: '/spend/:path*',
                destination: 'http://localhost:8081/spend/:path*',
            },
            {
                source: '/audit/:path*',
                destination: 'http://localhost:8081/audit/:path*',
            },
            {
                source: '/global/:path*',
                destination: 'http://localhost:8081/global/:path*',
            },
            {
                source: '/invitation/:path*',
                destination: 'http://localhost:8081/invitation/:path*',
            },
            {
                source: '/control/:path*',
                destination: 'http://localhost:8081/control/:path*',
            },
            {
                source: '/metrics',
                destination: 'http://localhost:8081/metrics',
            },
            {
                source: '/mcp/:path*',
                destination: 'http://localhost:8081/mcp/:path*',
            },
        ] : [];
    },
};

export default nextConfig;
