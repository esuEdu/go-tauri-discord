import { useRef, useState, type MouseEvent } from "react";
import type { SessionState } from "../session";
import type { Channel, Guild } from "../types/events.gen";
import { Avatar } from "../ui/Avatar";
import { ChannelRow } from "../ui/ChannelRow";
import { Icon } from "../ui/Icon";
import { LiveBadge } from "../ui/LiveBadge";

const THRESHOLD = 5;

export type ChannelMenuTarget =
  | { kind: "list" }
  | { kind: "channel"; channel: Channel }
  | { kind: "category"; name: string };

export type ChannelMove = {
  channelID: string;
  parentID: string | null;
  position: number;
};

type Drag = {
  channel: Channel;
  x: number;
  y: number;
  grabX: number;
  grabY: number;
  lifted: boolean;
};

type Slot = { parent: string | null; index: number; top: number };

export function Channels({
  guild,
  channels,
  activeChannelID,
  state,
  meID,
  live,
  speaking,
  canManage,
  avatarURL,
  onPick,
  onOpenServerMenu,
  onOpenMenu,
  onOpenVoiceMember,
  onMoveChannel,
}: {
  guild: Guild;
  channels: Channel[];
  activeChannelID: string | null;
  state: SessionState;
  meID: string;
  live: Set<string>;
  speaking: Record<string, boolean>;
  canManage: boolean;
  avatarURL: (id: string) => string | null;
  onPick: (channel: Channel) => void;
  onOpenServerMenu: (event: MouseEvent<HTMLButtonElement>) => void;
  onOpenMenu: (target: ChannelMenuTarget, event: MouseEvent<HTMLElement>) => void;
  onOpenVoiceMember: (
    userID: string,
    channelID: string,
    event: MouseEvent<HTMLElement>,
  ) => void;
  onMoveChannel: (move: ChannelMove) => void;
}) {
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  const [drag, setDrag] = useState<Drag | null>(null);
  const [slot, setSlot] = useState<Slot | null>(null);
  const list = useRef<HTMLDivElement>(null);
  const rows = useRef(new Map<string, HTMLElement>());
  const heads = useRef(new Map<string, HTMLElement>());
  const clickable = useRef(true);

  const members = state.membersByGuild[guild.id] ?? [];
  const onlineCount = members.filter((id) => state.online[id]).length;

  const byPosition = (a: Channel, b: Channel) => a.position - b.position;
  const top = channels.filter((c) => !c.parent_id).sort(byPosition);
  const childrenOf = (id: string) =>
    channels.filter((c) => c.parent_id === id).sort(byPosition);

  function groups(): { parent: string | null; ids: string[] }[] {
    const out: { parent: string | null; ids: string[] }[] = [
      { parent: null, ids: top.filter((c) => c.kind !== "category").map((c) => c.id) },
    ];
    for (const item of top) {
      if (item.kind !== "category") continue;
      out.push({ parent: item.id, ids: childrenOf(item.id).map((c) => c.id) });
    }
    return out;
  }

  function slots(): Slot[] {
    const box = list.current?.getBoundingClientRect();
    if (!box) return [];
    const scroll = list.current!.scrollTop;
    const found: Slot[] = [];

    for (const group of groups()) {
      const head = group.parent ? heads.current.get(group.parent) : null;
      if (group.ids.length === 0) {
        if (head) {
          const r = head.getBoundingClientRect();
          found.push({ parent: group.parent, index: 0, top: r.bottom - box.top + scroll });
        }
        continue;
      }
      group.ids.forEach((id, index) => {
        const node = rows.current.get(id);
        if (!node) return;
        const r = node.getBoundingClientRect();
        found.push({ parent: group.parent, index, top: r.top - box.top + scroll - 2 });
        if (index === group.ids.length - 1) {
          found.push({
            parent: group.parent,
            index: index + 1,
            top: r.bottom - box.top + scroll - 1,
          });
        }
      });
    }
    return found;
  }

  function nearest(clientY: number): Slot | null {
    const box = list.current?.getBoundingClientRect();
    if (!box) return null;
    const y = clientY - box.top + list.current!.scrollTop;
    let best: Slot | null = null;
    let gap = Infinity;
    for (const candidate of slots()) {
      const distance = Math.abs(candidate.top - y);
      if (distance < gap) {
        gap = distance;
        best = candidate;
      }
    }
    return best;
  }

  function stop() {
    setDrag(null);
    setSlot(null);
  }

  function dragProps(channel: Channel) {
    if (!canManage) return {};
    return {
      draggable: false,
      onDragStart: (event: React.DragEvent) => event.preventDefault(),
      onPointerDown: (event: React.PointerEvent<HTMLElement>) => {
        if (event.button !== 0) return;
        clickable.current = true;
        event.currentTarget.setPointerCapture(event.pointerId);
        setDrag({
          channel,
          x: event.clientX,
          y: event.clientY,
          grabX: event.clientX,
          grabY: event.clientY,
          lifted: false,
        });
      },
      onPointerMove: (event: React.PointerEvent<HTMLElement>) => {
        if (!drag || drag.channel.id !== channel.id) return;
        const far =
          Math.abs(event.clientX - drag.grabX) >= THRESHOLD ||
          Math.abs(event.clientY - drag.grabY) >= THRESHOLD;
        if (!drag.lifted && !far) return;
        clickable.current = false;
        setDrag({ ...drag, x: event.clientX, y: event.clientY, lifted: true });
        setSlot(nearest(event.clientY));
      },
      onPointerUp: (event: React.PointerEvent<HTMLElement>) => {
        if (event.currentTarget.hasPointerCapture(event.pointerId)) {
          event.currentTarget.releasePointerCapture(event.pointerId);
        }
        if (drag?.lifted && slot) {
          const from = drag.channel;
          const here = from.parent_id ?? null;
          const sameGroup = here === slot.parent;
          const siblings = groups().find((g) => g.parent === slot.parent)?.ids ?? [];
          const at = siblings.indexOf(from.id);
          let index = slot.index;
          if (sameGroup && at !== -1 && slot.index > at) index -= 1;
          if (!sameGroup || index !== at) {
            onMoveChannel({ channelID: from.id, parentID: slot.parent, position: index });
          }
        }
        stop();
      },
      onPointerCancel: stop,
    };
  }

  function channelRow(channel: Channel) {
    const inCall = state.inVoice[channel.id] ?? [];
    const unread = state.unread[channel.id];
    const active = channel.id === activeChannelID;
    return (
      <div
        key={channel.id}
        ref={(node) => {
          if (node) rows.current.set(channel.id, node);
          else rows.current.delete(channel.id);
        }}
      >
        <ChannelRow
          kind={channel.kind === "voice" ? "voice" : "text"}
          name={channel.name}
          count={inCall.length}
          lifted={drag?.channel.id === channel.id && drag.lifted}
          state={active ? "active" : inCall.length ? "busy" : unread ? "unread" : "idle"}
          onClick={() => {
            if (!clickable.current) {
              clickable.current = true;
              return;
            }
            onPick(channel);
          }}
          onContextMenu={(event) => onOpenMenu({ kind: "channel", channel }, event)}
          {...dragProps(channel)}
        />
        {channel.kind === "voice" && inCall.length > 0 && (
          <div className="voice-members">
            {inCall.map((id) => (
              <div
                key={id}
                className="voice-member"
                role="button"
                tabIndex={0}
                onContextMenu={(event) => onOpenVoiceMember(id, channel.id, event)}
              >
                <Avatar
                  name={state.names[id] ?? "?"}
                  url={avatarURL(id)}
                  size={20}
                  tone={id === meID ? "accent" : "neutral"}
                  className={speaking[id] ? "speaking-ring" : undefined}
                />
                <span className="voice-member-name">{state.names[id] ?? "Someone"}</span>
                {live.has(id) && <LiveBadge tone="solid" />}
                {id === meID && <span className="voice-member-you">you</span>}
                <span className="voice-member-icons">
                  {state.mutedInVoice[id] && (
                    <Icon name="microphone-slash" size={12} tone="bad" />
                  )}
                  {state.deafenedInVoice[id] && (
                    <Icon name="headphones-slash" size={12} tone="bad" />
                  )}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
    );
  }

  return (
    <div className="channels panel">
      <button type="button" className="server-head" onClick={onOpenServerMenu}>
        <span className="server-head-text">
          <span className="server-head-name">{guild.name}</span>
          <span className="server-head-meta">
            {members.length} {members.length === 1 ? "member" : "members"} · {onlineCount}{" "}
            online
          </span>
        </span>
        <Icon name="caret-down" size={15} />
      </button>

      <div
        className="channel-list"
        ref={list}
        onContextMenu={(event) => {
          if (event.target === event.currentTarget) onOpenMenu({ kind: "list" }, event);
        }}
      >
        {drag?.lifted && slot && (
          <span className="channel-drop-line" style={{ top: slot.top }} />
        )}

        {top.map((item) =>
          item.kind === "category" ? (
            <div key={item.id}>
              <button
                type="button"
                className="category"
                data-collapsed={collapsed[item.id]}
                ref={(node) => {
                  if (node) heads.current.set(item.id, node);
                  else heads.current.delete(item.id);
                }}
                onClick={() =>
                  setCollapsed((was) => ({ ...was, [item.id]: !was[item.id] }))
                }
                onContextMenu={(event) =>
                  onOpenMenu({ kind: "category", name: item.name }, event)
                }
              >
                <Icon name="caret-down" size={11} className="category-caret" />
                {item.name}
              </button>
              {!collapsed[item.id] && childrenOf(item.id).map(channelRow)}
            </div>
          ) : (
            channelRow(item)
          ),
        )}
      </div>

      {drag?.lifted && (
        <span className="channel-ghost" style={{ left: drag.x, top: drag.y }}>
          <Icon name={drag.channel.kind === "voice" ? "speaker-high" : "hash"} size={15} />
          <span className="channel-ghost-name">{drag.channel.name}</span>
        </span>
      )}
    </div>
  );
}
