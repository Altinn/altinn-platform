import { Field, Label, Select, Tabs } from '@digdir/designsystemet-react';
import { envLabel } from '../lib/flux';

interface Props {
  tenants: string[];
  activeTenant: string;
  onTenant: (tenant: string) => void;
  envs: string[];
  activeEnv: string;
  onEnv: (env: string) => void;
}

/** The tenant + environment scope selector shared by Home and the per-product
 *  DIS resource views — like a cloud console's project/region picker, it stays
 *  put as you move between products. */
export function DisScope({ tenants, activeTenant, onTenant, envs, activeEnv, onEnv }: Props) {
  return (
    <div className="disscope">
      {tenants.length > 1 && (
        <div className="matrix__tenants">
          <span className="matrix__tenants-label">Tenant</span>
          <Tabs value={activeTenant} onChange={onTenant} data-size="sm">
            <Tabs.List>
              {tenants.map((t) => (
                <Tabs.Tab key={t} value={t}>
                  {t}
                </Tabs.Tab>
              ))}
            </Tabs.List>
          </Tabs>
        </div>
      )}

      <div className="matrix__filters">
        <Field>
          <Label data-size="sm">Environment</Label>
          <Select
            value={activeEnv}
            onChange={(e) => onEnv(e.target.value)}
            data-size="sm"
            width="auto"
          >
            {envs.map((e) => (
              <Select.Option key={e} value={e}>
                {envLabel(e)}
              </Select.Option>
            ))}
          </Select>
        </Field>
      </div>
    </div>
  );
}
