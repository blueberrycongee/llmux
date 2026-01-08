/** @type {import('next').NextConfig} */
const nextConfig = {
    output: 'export',
    distDir: '../cmd/server/ui_assets',
    trailingSlash: true,
    images: { unoptimized: true },

    // Development proxy to avoid CORS
    async rewrites() {
        return process.env.NODE_ENV === 'development' ? [
            {
                source: '/api/v1/:path*',
                destination: 'http://localhost:8080/api/v1/:path*',
            },
        ] : [];
    },
};

export default nextConfig;
