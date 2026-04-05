import React from 'react';

// Canvas: 1440 x 810 (16:9)
const W = 1440;
const H = 810;

// DAG nodes — 5 layers, left to right
const nodes = [
  // Layer 0
  { id: 'A', x: 80,   y: 200 },
  { id: 'B', x: 80,   y: 560 },
  // Layer 1
  { id: 'C', x: 290,  y: 105 },
  { id: 'D', x: 290,  y: 380 },
  { id: 'E', x: 290,  y: 670 },
  // Layer 2
  { id: 'F', x: 530,  y: 210 },
  { id: 'G', x: 530,  y: 490 },
  { id: 'H', x: 530,  y: 740 },
  // Layer 3
  { id: 'I', x: 760,  y: 140 },
  { id: 'J', x: 760,  y: 420 },
  { id: 'K', x: 760,  y: 700 },
  // Layer 4
  { id: 'L', x: 980,  y: 260 },
  { id: 'M', x: 980,  y: 570 },
  // Layer 5
  { id: 'N', x: 1200, y: 155 },
  { id: 'O', x: 1200, y: 420 },
  { id: 'P', x: 1200, y: 670 },
  // Layer 6
  { id: 'Q', x: 1390, y: 320 },
];

const nMap = Object.fromEntries(nodes.map(n => [n.id, n]));

// Edges forming a workflow DAG
const edgeDefs = [
  { from: 'A', to: 'C' }, { from: 'A', to: 'D' },
  { from: 'B', to: 'D' }, { from: 'B', to: 'E' },
  { from: 'C', to: 'F' },
  { from: 'D', to: 'F' }, { from: 'D', to: 'G' },
  { from: 'E', to: 'G' }, { from: 'E', to: 'H' },
  { from: 'F', to: 'I' }, { from: 'F', to: 'J' },
  { from: 'G', to: 'J' }, { from: 'G', to: 'K' },
  { from: 'H', to: 'K' },
  { from: 'I', to: 'L' },
  { from: 'J', to: 'L' }, { from: 'J', to: 'M' },
  { from: 'K', to: 'M' },
  { from: 'L', to: 'N' }, { from: 'L', to: 'O' },
  { from: 'M', to: 'O' }, { from: 'M', to: 'P' },
  { from: 'N', to: 'Q' },
  { from: 'O', to: 'Q' },
  { from: 'P', to: 'Q' },
];

// Staggered animation timings
const durations = [3.2, 4.0, 3.6, 5.0, 4.4, 3.8, 4.8, 3.4, 5.2, 4.2,
                   3.0, 4.6, 3.8, 5.4, 4.0, 3.2, 4.8, 3.6, 5.0, 3.4,
                   4.2, 5.6, 3.8, 4.4, 3.0];
const delays   = [0, 1.2, 2.4, 0.6, 1.8, 3.0, 0.4, 2.0, 1.4, 3.2,
                  0.8, 2.6, 0.2, 1.6, 3.4, 0.6, 2.2, 1.0, 3.8, 0.4,
                  2.8, 1.2, 0.8, 2.0, 1.6];

// Nodes that have a visible pulsing ring (the "important" ones)
const pulseNodes = new Set(['A', 'D', 'G', 'J', 'L', 'O', 'Q']);

// Offset delay for pulse so they don't all sync
const pulseDelays: Record<string, number> = {
  A: 0, D: 0.8, G: 1.6, J: 0.4, L: 1.2, O: 2.0, Q: 0.6,
};

export default function BackgroundGraph(): React.JSX.Element {
  return (
    <svg
      style={{
        position: 'fixed',
        inset: 0,
        width: '100%',
        height: '100%',
        pointerEvents: 'none',
        zIndex: 0,
      }}
      viewBox={`0 0 ${W} ${H}`}
      preserveAspectRatio="xMidYMid slice"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
    >
      <defs>
        {/* Edge paths used by animateMotion */}
        {edgeDefs.map((e, i) => {
          const a = nMap[e.from];
          const b = nMap[e.to];
          // Slight cubic bezier for organic feel
          const mx = (a.x + b.x) / 2;
          const my = (a.y + b.y) / 2;
          return (
            <path
              key={`def-${i}`}
              id={`ep-${i}`}
              d={`M${a.x},${a.y} Q${mx},${a.y} ${b.x},${b.y}`}
            />
          );
        })}
      </defs>

      {/* ── Edges ── */}
      <g className="bg-edges">
        {edgeDefs.map((e, i) => {
          const a = nMap[e.from];
          const b = nMap[e.to];
          const mx = (a.x + b.x) / 2;
          return (
            <path
              key={`edge-${i}`}
              d={`M${a.x},${a.y} Q${mx},${a.y} ${b.x},${b.y}`}
              fill="none"
              className="bg-edge"
            />
          );
        })}
      </g>

      {/* ── Particles flowing along edges ── */}
      <g className="bg-particles">
        {edgeDefs.map((_, i) => (
          <circle key={`p-${i}`} r={3} className="bg-particle">
            <animateMotion
              dur={`${durations[i] ?? 4}s`}
              begin={`${delays[i] ?? 0}s`}
              repeatCount="indefinite"
            >
              <mpath href={`#ep-${i}`} />
            </animateMotion>
          </circle>
        ))}
      </g>

      {/* ── Nodes ── */}
      <g className="bg-nodes">
        {nodes.map((n) => (
          <g key={n.id}>
            {/* Pulse ring on key nodes */}
            {pulseNodes.has(n.id) && (
              <circle
                cx={n.x}
                cy={n.y}
                r={10}
                fill="none"
                className="bg-pulse"
                style={{ animationDelay: `${pulseDelays[n.id] ?? 0}s` }}
              />
            )}
            {/* Node dot */}
            <circle
              cx={n.x}
              cy={n.y}
              r={5}
              className="bg-node"
            />
          </g>
        ))}
      </g>
    </svg>
  );
}
