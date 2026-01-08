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
    trailingSlash: true,
    images: { unoptimized: true },

    // Development proxy to avoid CORS
    async rewrites() {
        return !isProd ? [
            {
                source: '/api/v1/:path*',
                destination: 'http://localhost:8080/api/v1/:path*',
            },
        ] : [];
    },
};

export default nextConfig;
