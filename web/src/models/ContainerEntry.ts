
export default interface ContainerEntry {
    ContainerId: string;
    ContainerName: string;
    ImageName: string;
    ImageNameShort: string;
    ImageVersion: string;
    ImageCreatedDate: string;
    NewVersion: string;
    NewVersionCreated: string;
    HasUpdate: boolean;
    Selected: boolean;
    IsChecking: boolean;
}