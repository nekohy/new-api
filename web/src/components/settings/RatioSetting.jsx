/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useState, useEffect } from 'react';
import { Card, Tabs, TabPane } from '@douyinfe/semi-ui';
import { useNavigate, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Users, Package } from 'lucide-react';

import GroupPricing from '../../pages/Setting/Pricing/GroupPricing';
import UserGroupSettings from '../../pages/Setting/Pricing/UserGroupSettings';

const RatioSetting = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const [subTab, setSubTab] = useState('model');

  useEffect(() => {
    const searchParams = new URLSearchParams(location.search);
    const sub = searchParams.get('sub');
    if (sub === 'user' || sub === 'model') {
      setSubTab(sub);
    } else {
      // 默认显示模型分组
      setSubTab('model');
    }
  }, [location.search]);

  const onChangeSubTab = (key) => {
    setSubTab(key);
    const searchParams = new URLSearchParams(location.search);
    searchParams.set('sub', key);
    navigate(`?${searchParams.toString()}`, { replace: true });
  };

  return (
    <Card style={{ marginTop: '10px' }}>
      <Tabs
        type="line"
        activeKey={subTab}
        onChange={onChangeSubTab}
        style={{ marginBottom: 16 }}
      >
        <TabPane
          tab={
            <span style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
              <Package size={16} />
              {t('模型分组')}
            </span>
          }
          itemKey="model"
        >
          {subTab === 'model' && <GroupPricing />}
        </TabPane>
        <TabPane
          tab={
            <span style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
              <Users size={16} />
              {t('用户分组')}
            </span>
          }
          itemKey="user"
        >
          {subTab === 'user' && <UserGroupSettings />}
        </TabPane>
      </Tabs>
    </Card>
  );
};

export default RatioSetting;
