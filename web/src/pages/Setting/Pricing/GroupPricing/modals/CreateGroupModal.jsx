import React from 'react';
import { Modal, Input } from '@douyinfe/semi-ui';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

const CreateGroupModal = ({ visible, onOk, onCancel, loading }) => {
  const { t } = useTranslation();
  const [newGroupName, setNewGroupName] = React.useState('');

  const handleOk = () => {
    if (!newGroupName.trim()) return;
    onOk(newGroupName.trim());
  };

  const handleCancel = () => {
    setNewGroupName('');
    onCancel();
  };

  // 关闭后重置
  React.useEffect(() => {
    if (!visible) {
      setNewGroupName('');
    }
  }, [visible]);

  return (
    <Modal
      title={t('创建定价分组')}
      visible={visible}
      onOk={handleOk}
      onCancel={handleCancel}
      okText={t('创建')}
      cancelText={t('取消')}
      confirmLoading={loading}
    >
      <Input
        placeholder={t('输入分组名称')}
        value={newGroupName}
        onChange={setNewGroupName}
      />
    </Modal>
  );
};

CreateGroupModal.propTypes = {
  visible: PropTypes.bool.isRequired,
  onOk: PropTypes.func.isRequired,
  onCancel: PropTypes.func.isRequired,
  loading: PropTypes.bool,
};

export default CreateGroupModal;
