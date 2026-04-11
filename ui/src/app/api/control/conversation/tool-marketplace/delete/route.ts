import { NextRequest, NextResponse } from "next/server";
const MANAGEMENT_API_BASE = process.env.MANAGEMENT_API_BASE || "http://127.0.0.1:8081";
export async function POST(request: NextRequest) { const body = await request.text(); const response = await fetch(`${MANAGEMENT_API_BASE}/control/conversation/tool-marketplace/delete`, { method: "POST", headers: { "Content-Type": "application/json" }, body }); const data = await response.json(); return NextResponse.json(data, { status: response.status }); }
