import type { Context } from '@netlify/functions'
import { createClient } from '@supabase/supabase-js'

const ALLOWED_PHONES = ['3104861833', '6264043580']
const MAX_BODY_LENGTH = 10_000
const RATE_LIMIT_PER_HOUR = 10

function normalizePhone(input: string): string {
  return input.replace(/\D/g, '').replace(/^1(\d{10})$/, '$1')
}

function getSupabase() {
  const url = process.env.SUPABASE_URL
  const key = process.env.SUPABASE_SERVICE_ROLE_KEY
  if (!url || !key) throw new Error('Missing Supabase config')
  return createClient(url, key)
}

function corsHeaders() {
  return {
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Headers': 'Content-Type',
    'Access-Control-Allow-Methods': 'GET, POST, OPTIONS',
  }
}

export default async (req: Request, _context: Context) => {
  // CORS preflight
  if (req.method === 'OPTIONS') {
    return new Response(null, { status: 204, headers: corsHeaders() })
  }

  const headers = { ...corsHeaders(), 'Content-Type': 'application/json' }

  try {
    if (req.method === 'POST') {
      return await handleSubmit(req, headers)
    }

    if (req.method === 'GET') {
      return await handleList(req, headers)
    }

    return new Response(JSON.stringify({ error: 'Method not allowed' }), {
      status: 405,
      headers,
    })
  } catch (err) {
    console.error('Feedback function error:', err)
    return new Response(JSON.stringify({ error: 'Something went wrong.' }), {
      status: 500,
      headers,
    })
  }
}

async function handleSubmit(req: Request, headers: Record<string, string>) {
  const { phone, body, urgency } = await req.json()

  // Validate phone
  const normalized = normalizePhone(phone || '')
  if (!ALLOWED_PHONES.includes(normalized)) {
    return new Response(JSON.stringify({ error: 'Not authorized.' }), {
      status: 403,
      headers,
    })
  }

  // Validate body
  const text = (body || '').trim()
  if (!text) {
    return new Response(JSON.stringify({ error: 'Feedback cannot be empty.' }), {
      status: 400,
      headers,
    })
  }
  if (text.length > MAX_BODY_LENGTH) {
    return new Response(JSON.stringify({ error: 'Feedback is too long.' }), {
      status: 400,
      headers,
    })
  }

  // Validate urgency
  const urg = urgency === 'critical' ? 'critical' : 'normal'

  const supabase = getSupabase()

  // Rate limit: max 10 per phone per hour
  const oneHourAgo = new Date(Date.now() - 60 * 60 * 1000).toISOString()
  const { count } = await supabase
    .from('feedback_requests')
    .select('id', { count: 'exact', head: true })
    .eq('phone', normalized)
    .gte('created_at', oneHourAgo)

  if ((count ?? 0) >= RATE_LIMIT_PER_HOUR) {
    return new Response(
      JSON.stringify({
        error: 'Easy there — max 10 submissions per hour. Take a breather.',
      }),
      { status: 429, headers }
    )
  }

  // Insert
  const { data, error } = await supabase
    .from('feedback_requests')
    .insert({ phone: normalized, body: text, urgency: urg })
    .select('id')
    .single()

  if (error) {
    console.error('Supabase insert error:', error)
    return new Response(JSON.stringify({ error: 'Failed to save feedback.' }), {
      status: 500,
      headers,
    })
  }

  return new Response(JSON.stringify({ success: true, id: data.id }), {
    status: 201,
    headers,
  })
}

async function handleList(req: Request, headers: Record<string, string>) {
  const url = new URL(req.url)
  const phone = url.searchParams.get('phone') || ''
  const normalized = normalizePhone(phone)

  if (!ALLOWED_PHONES.includes(normalized)) {
    return new Response(JSON.stringify({ error: 'Not authorized.' }), {
      status: 403,
      headers,
    })
  }

  const supabase = getSupabase()
  const { data, error } = await supabase
    .from('feedback_requests')
    .select('id, body, urgency, status, admin_notes, created_at')
    .eq('phone', normalized)
    .order('created_at', { ascending: false })
    .limit(50)

  if (error) {
    console.error('Supabase select error:', error)
    return new Response(JSON.stringify({ error: 'Failed to load feedback.' }), {
      status: 500,
      headers,
    })
  }

  return new Response(JSON.stringify(data), { status: 200, headers })
}
