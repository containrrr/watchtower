import ContainerModel from "../models/ContainerModel";
import ImageInfo from "./ImageInfo";
import SpinnerGrow from "./SpinnerGrow";

interface ContainerListEntryProps extends ContainerModel {
    onClick: () => void;
}

const ContainerListEntry = (props: ContainerListEntryProps) => (
    <li className="list-group-item d-flex justify-content-between align-items-center container-list-entry" onClick={props.onClick} role="button">
        <div className="ms-1 me-3 container-list-entry-icon">
            {props.Selected
                ? <i className="bi bi-box-fill text-primary fs-4"></i>
                : <i className="bi bi-box text-muted fs-4"></i>
            }
        </div>
        <div className="me-auto">
            <div className="fw-bold">{props.ContainerName}</div>
            <span className="user-select-all">{props.ImageName}</span> <ImageInfo version={props.ImageVersion} created={props.ImageCreatedDate} />
        </div>
        <div className="float-end d-flex align-items-center">
            {props.HasUpdate === true && <ImageInfo version={props.NewVersion} created={props.NewVersionCreated} />}
            {props.IsChecking
                ? <SpinnerGrow />
                : props.HasUpdate === true && <i className="bi bi-arrow-down-circle-fill fs-4 text-primary"></i>
            }
        </div>
    </li>
);

export default ContainerListEntry;