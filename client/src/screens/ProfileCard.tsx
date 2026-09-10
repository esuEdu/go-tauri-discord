import { useEffect, useState } from "react";
import { api, type MemberProfile } from "../api";
import { Avatar } from "../ui/Avatar";
import { ContextMenu, type Anchor } from "../ui/ContextMenu";

export function ProfileCard({
  at,
  guildID,
  userID,
  status,
  saying,
  avatarURL,
  onClose,
}: {
  at: Anchor;
  guildID: string;
  userID: string;
  status: string;
  saying: string | null;
  avatarURL: string | null;
  onClose: () => void;
}) {
  const [profile, setProfile] = useState<MemberProfile | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let live = true;
    setProfile(null);
    setFailed(false);
    api
      .memberProfile(guildID, userID)
      .then((got) => {
        if (live) setProfile(got);
      })
      .catch(() => {
        if (live) setFailed(true);
      });
    return () => {
      live = false;
    };
  }, [guildID, userID]);

  return (
    <ContextMenu at={at} width={264} role="dialog" onClose={onClose}>
      <div className="person-card">
        <div className="person-head">
          <span className="avatar-slot">
            <Avatar name={profile?.user.username ?? "?"} url={avatarURL} size={56} />
            <span className="presence-dot" data-status={status} />
          </span>
          <span className="person-identity">
            <span className="person-name">{profile?.user.username ?? "…"}</span>
            {profile && (
              <span className="person-tag">#{profile.user.discriminator}</span>
            )}
          </span>
        </div>

        {saying && <p className="person-saying">{saying}</p>}

        {failed && <p className="person-empty">That profile could not be loaded.</p>}

        {profile && (
          <>
            {profile.bio ? (
              <p className="person-bio">{profile.bio}</p>
            ) : (
              <p className="person-empty">Nothing written yet.</p>
            )}

            <span className="person-kicker">Roles</span>
            {profile.roles.length ? (
              <span className="person-roles">
                {profile.roles.map((role) => (
                  <span key={role.id} className="person-role">
                    {role.name}
                  </span>
                ))}
              </span>
            ) : (
              <p className="person-empty">None yet.</p>
            )}

            <span className="person-kicker">Here since</span>
            <span className="person-joined">{whenJoined(profile.joined_at)}</span>
          </>
        )}
      </div>
    </ContextMenu>
  );
}

function whenJoined(iso: string): string {
  const at = new Date(iso);
  if (Number.isNaN(at.getTime())) return "Unknown";
  return at.toLocaleDateString(undefined, {
    year: "numeric",
    month: "long",
    day: "numeric",
  });
}
