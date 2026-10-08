import { useEffect, useState } from 'react'
import { FONT } from '../styles'
import { useAvatarVersion } from '../avatarStore'

interface UserAvatarProps {
  userId: number | null
  initials: string
  size: number
}

/**
 * Circle showing the user's photo, or their initials when there is none (404 from
 * /api/avatar). The initials render underneath; the photo shows only once it has
 * loaded, so a missing photo never flashes a broken image.
 */
export function UserAvatar({ userId, initials, size }: UserAvatarProps) {
  const version = useAvatarVersion()
  const [loaded, setLoaded] = useState(false)
  const [failed, setFailed] = useState(false)

  useEffect(() => { setLoaded(false); setFailed(false) }, [version, userId])

  return (
    <span
      data-testid="user-avatar"
      style={{
        position: 'relative', display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
        width: size, height: size, borderRadius: '50%', overflow: 'hidden', flexShrink: 0,
        backgroundColor: 'var(--brand)', color: '#fff',
        fontFamily: FONT, fontWeight: 700, fontSize: Math.round(size * 0.4),
      }}
    >
      {initials ? initials : (
        <svg width={size * 0.55} height={size * 0.55} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>
        </svg>
      )}
      {userId && !failed && (
        <img
          key={version}
          src={`/api/avatar/${userId}?v=${version}`}
          alt=""
          onLoad={() => setLoaded(true)}
          onError={() => setFailed(true)}
          style={{
            position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: 'cover',
            opacity: loaded ? 1 : 0,
          }}
        />
      )}
    </span>
  )
}
