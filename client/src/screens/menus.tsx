import { useState } from "react";
import { MAX_VOLUME, voice } from "../voice";
import type { Channel } from "../types/events.gen";
import { Avatar } from "../ui/Avatar";
import { ContextMenu, type Anchor } from "../ui/ContextMenu";
import { LiveBadge } from "../ui/LiveBadge";
import { MenuItem, MenuSeparator } from "../ui/Menu";
import { VolumeSlider } from "../ui/VolumeSlider";

export function VoiceMemberMenu({
  at,
  userID,
  name,
  avatarURL,
  live,
  canKick,
  canBan,
  onKick,
  onBan,
  onClose,
}: {
  at: Anchor;
  userID: string;
  name: string;
  avatarURL: string | null;
  live: boolean;
  canKick: boolean;
  canBan: boolean;
  onKick: () => void;
  onBan: () => void;
  onClose: () => void;
}) {
  const [userVolume, setUserVolume] = useState(() => voice.volumeOf(userID, "voice"));
  const [streamVolume, setStreamVolume] = useState(() =>
    voice.volumeOf(userID, "screen"),
  );

  function setUser(level: number) {
    setUserVolume(level);
    voice.setVolume(userID, "voice", level);
  }

  function setStream(level: number) {
    setStreamVolume(level);
    voice.setVolume(userID, "screen", level);
  }

  return (
    <ContextMenu at={at} onClose={onClose}>
      <div className="menu-header">
        <Avatar name={name} url={avatarURL} size={28} />
        <span className="menu-header-name">{name}</span>
        {live && <LiveBadge />}
      </div>
      <MenuSeparator />

      <MenuItem
        icon={userVolume === 0 ? "microphone-slash" : "microphone"}
        label={userVolume === 0 ? "Unmute" : "Mute"}
        onClick={() => setUser(userVolume === 0 ? 1 : 0)}
      />
      <MenuItem icon="headphones" label="Deafen" hint="Needs the server" disabled />

      <VolumeSlider
        label="User volume"
        value={userVolume}
        max={MAX_VOLUME}
        onChange={setUser}
      />
      {live && (
        <VolumeSlider
          label="Stream volume"
          value={streamVolume}
          max={MAX_VOLUME}
          accentValue
          onChange={setStream}
        />
      )}

      <MenuSeparator />
      <MenuItem icon="user-circle" label="View Profile" hint="Not built yet" disabled />
      <MenuItem
        icon="pencil-simple"
        label="Change Nickname"
        hint="No endpoint yet"
        disabled
      />
      <MenuItem icon="phone-x" label="Disconnect" hint="Needs the server" disabled />

      {(canKick || canBan) && <MenuSeparator />}
      {canKick && (
        <MenuItem
          icon="phone-x"
          kind="danger"
          label="Kick from Server"
          onClick={() => {
            onClose();
            onKick();
          }}
        />
      )}
      {canBan && (
        <MenuItem
          icon="trash"
          kind="danger"
          label="Ban from Server"
          onClick={() => {
            onClose();
            onBan();
          }}
        />
      )}
    </ContextMenu>
  );
}

export function PersonMenu({
  at,
  canKick,
  canBan,
  onKick,
  onBan,
  onClose,
}: {
  at: Anchor;
  canKick: boolean;
  canBan: boolean;
  onKick: () => void;
  onBan: () => void;
  onClose: () => void;
}) {
  if (!canKick && !canBan) return null;
  return (
    <ContextMenu at={at} onClose={onClose}>
      {canKick && (
        <MenuItem
          icon="phone-x"
          kind="danger"
          label="Kick"
          onClick={() => {
            onClose();
            onKick();
          }}
        />
      )}
      {canBan && (
        <MenuItem
          icon="trash"
          kind="danger"
          label="Ban"
          onClick={() => {
            onClose();
            onBan();
          }}
        />
      )}
    </ContextMenu>
  );
}

export function ChannelMenu({
  at,
  channel,
  onClose,
}: {
  at: Anchor;
  channel: Channel;
  onClose: () => void;
}) {
  return (
    <ContextMenu at={at} onClose={onClose}>
      <MenuItem
        icon="pencil-simple"
        label="Edit channel"
        hint="No endpoint yet"
        disabled
      />
      <MenuItem
        icon="speaker-high"
        label="Notifications"
        hint="Nothing notifies yet"
        disabled
      />
      <MenuSeparator />
      <MenuItem
        icon="trash"
        kind="danger"
        label={`Delete ${channel.name}`}
        hint="No endpoint yet"
        disabled
      />
    </ContextMenu>
  );
}

export function MessageMenu({
  at,
  mine,
  canDelete,
  onEdit,
  onDelete,
  onCopy,
  onClose,
}: {
  at: Anchor;
  mine: boolean;
  canDelete: boolean;
  onEdit: () => void;
  onDelete: () => void;
  onCopy: () => void;
  onClose: () => void;
}) {
  return (
    <ContextMenu at={at} width={200} onClose={onClose}>
      {mine && (
        <MenuItem
          icon="pencil-simple"
          label="Edit"
          onClick={() => {
            onClose();
            onEdit();
          }}
        />
      )}
      <MenuItem
        icon="copy"
        label="Copy text"
        onClick={() => {
          onClose();
          onCopy();
        }}
      />
      {canDelete && <MenuSeparator />}
      {canDelete && (
        <MenuItem
          icon="trash"
          kind="danger"
          label="Delete"
          onClick={() => {
            onClose();
            onDelete();
          }}
        />
      )}
    </ContextMenu>
  );
}

export function CategoryMenu({
  at,
  canManage,
  onNewChannel,
  onNewVoiceChannel,
  onClose,
}: {
  at: Anchor;
  canManage: boolean;
  onNewChannel: () => void;
  onNewVoiceChannel: () => void;
  onClose: () => void;
}) {
  return (
    <ContextMenu at={at} onClose={onClose}>
      {canManage && (
        <MenuItem
          icon="hash"
          label="Create channel"
          onClick={() => {
            onClose();
            onNewChannel();
          }}
        />
      )}
      {canManage && (
        <MenuItem
          icon="speaker-high"
          label="Create voice channel"
          onClick={() => {
            onClose();
            onNewVoiceChannel();
          }}
        />
      )}
      {canManage && <MenuSeparator />}
      <MenuItem
        icon="speaker-high"
        label="Notifications"
        hint="Nothing notifies yet"
        disabled
      />
    </ContextMenu>
  );
}

export function ChannelListMenu({
  at,
  canManage,
  onNewChannel,
  onNewCategory,
  onClose,
}: {
  at: Anchor;
  canManage: boolean;
  onNewChannel: () => void;
  onNewCategory: () => void;
  onClose: () => void;
}) {
  if (!canManage) return null;
  return (
    <ContextMenu at={at} onClose={onClose}>
      <MenuItem
        icon="plus"
        label="Create channel"
        onClick={() => {
          onClose();
          onNewChannel();
        }}
      />
      <MenuItem
        icon="folder-plus"
        label="Create category"
        onClick={() => {
          onClose();
          onNewCategory();
        }}
      />
    </ContextMenu>
  );
}
