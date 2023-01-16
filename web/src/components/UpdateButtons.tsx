
interface UpdateButtonProps {
    disabled: boolean;
    onClick: () => void;
}

export const UpdateSelected = (props: UpdateButtonProps) => (
    <button type="button" className="btn btn-primary me-2" disabled={props.disabled} onClick={props.onClick}>
        <i className="bi bi-arrow-down-circle me-2"></i>Update selected
    </button>
);

export const UpdateAll = (props: UpdateButtonProps) => (
    <button type="button" className="btn btn-primary me-2" disabled={props.disabled} onClick={props.onClick}>
        <i className="bi bi-arrow-down-circle me-2"></i>Update all
    </button>
);

export const UpdateCheck = (props: UpdateButtonProps) => (
    <button type="button" className="btn btn-outline-primary" disabled={props.disabled} onClick={props.onClick}>
        <i className="bi bi-arrow-repeat me-2"></i>Check for updates
    </button>
);