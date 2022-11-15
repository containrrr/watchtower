import ContainerModel from "../models/ContainerModel";
import ContainerListEntry from "./ContainerListEntry";

interface ContainerListProps {
    containers: ContainerModel[];
    onContainerClick: (container: ContainerModel) => void;
}

const ContainerList = (props: ContainerListProps) => (
    <ul className="list-group">
        {props.containers.map((c) => <ContainerListEntry {...c} key={c.ContainerID} onClick={() => props.onContainerClick(c)} />)}
    </ul >
);

export default ContainerList;