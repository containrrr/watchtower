import { CheckResponse, ContainerListEntry } from "../services/Api";

export default interface ContainerModel extends ContainerListEntry, CheckResponse {
    Selected: boolean;
    IsChecking: boolean;
}