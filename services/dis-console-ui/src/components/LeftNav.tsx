import {
  Code,
  Database,
  House,
  Layers,
  Rocket,
  User,
  Vault,
  type LucideIcon,
} from 'lucide-react';

export type Section =
  | 'home'
  | 'deployments'
  | 'syncroots'
  | 'databases'
  | 'identities'
  | 'apim'
  | 'vaults';

interface Props {
  active: Section;
  collapsed?: boolean;
}

interface Item {
  id: Section;
  label: string;
  hint: string;
  Icon: LucideIcon;
}

// Grouped like a cloud console: a top group, then a "DIS resources" section
// with one entry per DIS product family.
const GROUPS: { label?: string; items: Item[] }[] = [
  {
    items: [
      { id: 'home', label: 'Home', hint: 'Overview of your DIS resources', Icon: House },
      {
        id: 'deployments',
        label: 'Deployments',
        hint: 'Flux releases across environments',
        Icon: Rocket,
      },
      {
        id: 'syncroots',
        label: 'Syncroots',
        hint: 'What each syncroot applies, per environment',
        Icon: Layers,
      },
    ],
  },
  {
    label: 'DIS resources',
    items: [
      {
        id: 'databases',
        label: 'Databases',
        hint: 'Postgres servers and databases',
        Icon: Database,
      },
      {
        id: 'identities',
        label: 'Identities',
        hint: 'Managed / application identities',
        Icon: User,
      },
      { id: 'apim', label: 'APIM', hint: 'API Management APIs and backends', Icon: Code },
      { id: 'vaults', label: 'Vaults', hint: 'Key vaults', Icon: Vault },
    ],
  },
];

/** The left nav rail: a view per DIS product family,
 *  collapsible to an icon-only rail (labels become hover tooltips). */
export function LeftNav({ active, collapsed = false }: Props) {
  return (
    <nav className={collapsed ? 'leftnav leftnav--collapsed' : 'leftnav'} aria-label="Sections">
      {GROUPS.map((group, i) => (
        <div className="leftnav__group" key={group.label ?? `g${i}`}>
          {group.label && <div className="leftnav__section">{group.label}</div>}
          <ul>
            {group.items.map(({ id, label, hint, Icon }) => (
              <li key={id}>
                <a
                  className="leftnav__item"
                  aria-current={active === id ? 'page' : undefined}
                  title={collapsed ? label : hint}
                  href={`#/${id}`}
                >
                  <Icon className="leftnav__icon" size="1.5rem" aria-hidden />
                  <span className="leftnav__label">{label}</span>
                </a>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </nav>
  );
}
