import React, {type ReactNode} from 'react';
import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

type InterfaceApiTabsProps = {
  desktop: ReactNode;
  api: ReactNode;
};

export default function InterfaceApiTabs({desktop, api}: InterfaceApiTabsProps) {
  return (
    <Tabs groupId="akoflow-interface-api">
      <TabItem value="desktop" label="AkôFlow Desktop" default>
        {desktop}
      </TabItem>
      <TabItem value="api" label="API">
        {api}
      </TabItem>
    </Tabs>
  );
}
