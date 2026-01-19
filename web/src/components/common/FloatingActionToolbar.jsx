import React from 'react';
import { Button, Space, Typography } from '@douyinfe/semi-ui';
import PropTypes from 'prop-types';

const { Text } = Typography;

/**
 * 悬浮操作工具栏组件
 *
 * 用于显示未保存更改提示和相关操作按钮，固定在页面底部
 */
const FloatingActionToolbar = ({
  visible,
  message,
  count,
  actions = [],
}) => {
  if (!visible) return null;

  return (
    <div
      style={{
        position: 'fixed',
        bottom: 24,
        left: '50%',
        transform: 'translateX(-50%)',
        zIndex: 1000,
        backgroundColor: 'var(--semi-color-bg-1)',
        borderRadius: 8,
        boxShadow: '0 4px 12px rgba(0, 0, 0, 0.15)',
        padding: '12px 24px',
        display: 'flex',
        alignItems: 'center',
        gap: 16,
      }}
    >
      {message && (
        <Text type="warning">
          {typeof message === 'function' ? message(count) : message}
        </Text>
      )}
      <Space>
        {actions.map((action, index) => (
          <Button
            key={index}
            icon={action.icon}
            type={action.type || 'primary'}
            onClick={action.onClick}
            loading={action.loading}
            disabled={action.disabled}
          >
            {action.label}
          </Button>
        ))}
      </Space>
    </div>
  );
};

FloatingActionToolbar.propTypes = {
  visible: PropTypes.bool.isRequired,
  message: PropTypes.oneOfType([PropTypes.node, PropTypes.func]),
  count: PropTypes.number,
  actions: PropTypes.arrayOf(
    PropTypes.shape({
      label: PropTypes.string.isRequired,
      onClick: PropTypes.func.isRequired,
      type: PropTypes.string,
      icon: PropTypes.node,
      loading: PropTypes.bool,
      disabled: PropTypes.bool,
    })
  ),
};

export default FloatingActionToolbar;
