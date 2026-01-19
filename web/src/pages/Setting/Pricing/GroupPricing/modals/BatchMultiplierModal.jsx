import React, { useState } from 'react';
import { Modal, Input, Select, Banner, Typography } from '@douyinfe/semi-ui';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

const BatchMultiplierModal = ({
  visible,
  onOk,
  onCancel,
  groups,
  selectedGroup,
  selectedCount,
  loading,
}) => {
  const { t } = useTranslation();
  const [batchMultiplier, setBatchMultiplier] = useState(1);
  const [batchSyncSourceGroup, setBatchSyncSourceGroup] = useState('');

  const handleOk = () => {
    if (!batchSyncSourceGroup) return;
    onOk({
      sourceGroup: batchSyncSourceGroup,
      multiplier: batchMultiplier,
    });
  };

  const handleCancel = () => {
    setBatchMultiplier(1);
    setBatchSyncSourceGroup('');
    onCancel();
  };

  // 过滤掉当前选中的分组
  const availableGroups = groups.filter((g) => g !== selectedGroup);

  return (
    <Modal
      title={t('从指定分组同步价格')}
      visible={visible}
      onOk={handleOk}
      onCancel={handleCancel}
      okText={t('同步')}
      cancelText={t('取消')}
      okButtonProps={{ disabled: !batchSyncSourceGroup }}
      confirmLoading={loading}
    >
      <div style={{ marginBottom: 16 }}>
        <Text strong>{t('源分组')}</Text>
        <Select
          value={batchSyncSourceGroup}
          onChange={setBatchSyncSourceGroup}
          style={{ width: '100%', marginTop: 8 }}
          optionList={availableGroups.map((g) => ({ value: g, label: g }))}
          placeholder={t('选择源分组')}
          filter
        />
        <Text type="tertiary" size="small">
          {t('从选择的源分组获取模型价格')}
        </Text>
      </div>
      <div style={{ marginBottom: 16 }}>
        <Text strong>{t('价格倍率')}</Text>
        <Input
          type="number"
          value={batchMultiplier}
          onChange={(val) => setBatchMultiplier(parseFloat(val) || 1)}
          min={0.01}
          step={0.1}
          style={{ width: '100%', marginTop: 8 }}
        />
        <Text type="tertiary" size="small">
          {t('新价格 = 源分组价格 × 倍率，1 表示与源分组相同')}
        </Text>
      </div>
      <Banner
        type="info"
        description={t(
          '将从 {{source}} 分组同步 {{count}} 个模型的价格，倍率 {{multiplier}}',
          {
            source: batchSyncSourceGroup || '?',
            count: selectedCount,
            multiplier: batchMultiplier,
          }
        )}
      />
    </Modal>
  );
};

BatchMultiplierModal.propTypes = {
  visible: PropTypes.bool.isRequired,
  onOk: PropTypes.func.isRequired,
  onCancel: PropTypes.func.isRequired,
  groups: PropTypes.arrayOf(PropTypes.string).isRequired,
  selectedGroup: PropTypes.string,
  selectedCount: PropTypes.number,
  loading: PropTypes.bool,
};

export default BatchMultiplierModal;
