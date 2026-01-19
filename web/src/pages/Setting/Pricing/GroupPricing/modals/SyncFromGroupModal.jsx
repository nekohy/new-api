import React, { useState } from 'react';
import { Modal, Input, Select, Tag, Banner, Typography } from '@douyinfe/semi-ui';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

const SyncFromGroupModal = ({
  visible,
  onOk,
  onCancel,
  groups,
  selectedGroup,
  loading,
}) => {
  const { t } = useTranslation();
  const [syncSourceGroup, setSyncSourceGroup] = useState('default');
  const [syncMultiplier, setSyncMultiplier] = useState(1);
  const [syncMode, setSyncMode] = useState('all');

  const handleOk = () => {
    onOk({
      sourceGroup: syncSourceGroup,
      multiplier: syncMultiplier,
      mode: syncMode,
    });
  };

  const handleCancel = () => {
    setSyncMultiplier(1);
    setSyncMode('all');
    setSyncSourceGroup('default');
    onCancel();
  };

  // 过滤掉当前选中的分组
  const availableGroups = groups.filter((g) => g !== selectedGroup);

  return (
    <Modal
      title={t('同步模型价格')}
      visible={visible}
      onOk={handleOk}
      onCancel={handleCancel}
      okText={t('同步')}
      cancelText={t('取消')}
      confirmLoading={loading}
    >
      <div style={{ marginBottom: 16 }}>
        <Text strong>{t('源分组')}</Text>
        <Select
          value={syncSourceGroup}
          onChange={setSyncSourceGroup}
          style={{ width: '100%', marginTop: 8 }}
          optionList={availableGroups.map((g) => ({ value: g, label: g }))}
          placeholder={t('选择源分组')}
        />
        <Text type="tertiary" size="small">
          {t('从选择的源分组同步价格配置到当前分组')}
        </Text>
      </div>
      <div style={{ marginBottom: 16 }}>
        <Text strong>{t('价格倍率')}</Text>
        <Input
          type="number"
          value={syncMultiplier}
          onChange={(val) => setSyncMultiplier(parseFloat(val) || 1)}
          min={0.01}
          step={0.1}
          style={{ width: '100%', marginTop: 8 }}
        />
        <Text type="tertiary" size="small">
          {t('1 = 与源分组相同，1.5 = 源分组价格 × 1.5')}
        </Text>
      </div>
      <div style={{ marginBottom: 16 }}>
        <Text strong>{t('同步模式')}</Text>
        <div style={{ marginTop: 8 }}>
          <Tag
            color={syncMode === 'all' ? 'blue' : 'grey'}
            style={{ cursor: 'pointer', marginRight: 8 }}
            onClick={() => setSyncMode('all')}
          >
            {t('同步所有模型')}
          </Tag>
          <Tag
            color={syncMode === 'missing' ? 'blue' : 'grey'}
            style={{ cursor: 'pointer' }}
            onClick={() => setSyncMode('missing')}
          >
            {t('仅同步未配置的模型')}
          </Tag>
        </div>
      </div>
      <Banner
        type="info"
        description={
          syncMode === 'all'
            ? t('将覆盖分组内所有模型的价格，按 源分组价格 × {{multiplier}} 计算', {
                multiplier: syncMultiplier,
              })
            : t('仅为尚未配置的模型设置价格，按 源分组价格 × {{multiplier}} 计算', {
                multiplier: syncMultiplier,
              })
        }
      />
    </Modal>
  );
};

SyncFromGroupModal.propTypes = {
  visible: PropTypes.bool.isRequired,
  onOk: PropTypes.func.isRequired,
  onCancel: PropTypes.func.isRequired,
  groups: PropTypes.arrayOf(PropTypes.string).isRequired,
  selectedGroup: PropTypes.string,
  loading: PropTypes.bool,
};

export default SyncFromGroupModal;
